package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/gin-gonic/gin"
)

var PORT = 5002

const OTP_TTL = 600
const idChars = "abcdefghijklmnopqrstuvwxyz0123456789"

type SessionMeta struct {
	Number    string
	Name      string
	Proxy     string
	ExpiresAt int64
}

var (
	otpCache   = make(map[string]*SessionMeta)
	numberToID = make(map[string]string)
	cacheMu    = sync.RWMutex{}
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func generateID() string {
	b := make([]byte, 8)
	for i := range b {
		b[i] = idChars[rand.Intn(len(idChars))]
	}
	return string(b)
}

func storeSession(number, name, proxy string) string {
	cacheMu.Lock()
	oldID, ok := numberToID[number]
	if ok {
		delete(otpCache, oldID)
	}

	newID := generateID()
	for {
		if _, exists := otpCache[newID]; !exists {
			break
		}
		newID = generateID()
	}

	expiresAt := time.Now().Unix() + OTP_TTL
	otpCache[newID] = &SessionMeta{
		Number:    number,
		Name:      name,
		Proxy:     proxy,
		ExpiresAt: expiresAt,
	}
	numberToID[number] = newID
	cacheMu.Unlock()

	go func(id, num string, delay int) {
		time.Sleep(time.Duration(delay) * time.Second)
		cacheMu.Lock()
		defer cacheMu.Unlock()
		if _, exists := otpCache[id]; exists {
			delete(otpCache, id)
		}
		if numberToID[num] == id {
			delete(numberToID, num)
		}
	}(newID, number, OTP_TTL)

	return newID
}

func getSession(id string) *SessionMeta {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	meta, ok := otpCache[id]
	if !ok {
		return nil
	}
	if time.Now().Unix() > meta.ExpiresAt {
		delete(otpCache, id)
		if numberToID[meta.Number] == id {
			delete(numberToID, meta.Number)
		}
		return nil
	}
	return meta
}

func getBaseURL(c *gin.Context) string {
	scheme := c.GetHeader("x-forwarded-proto")
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := c.GetHeader("x-forwarded-host")
	if host == "" {
		host = c.GetHeader("host")
		if host == "" {
			host = c.Request.Host
		}
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

func fetchProxyList() []string {
	limit := rand.Intn(3000-15+1) + 15
	urlStr := fmt.Sprintf("https://api.proxyscrape.com/v4/free-proxy-list/get?request=displayproxies&protocol=http&timeout=10000&country=all&ssl=all&anonymity=all&skip=0&limit=%d", limit)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(urlStr)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")

	var proxies []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, ":") {
			proxies = append(proxies, line)
			if len(proxies) == 15 {
				break
			}
		}
	}
	return proxies
}

func testProxiesParallel(proxies []string) string {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 15)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var foundProxy string
	var mu sync.Mutex

	for _, proxy := range proxies {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			mu.Lock()
			if foundProxy != "" {
				mu.Unlock()
				return
			}
			mu.Unlock()

			proxyURL, err := url.Parse("http://" + p)
			if err != nil {
				return
			}

			transport := &http.Transport{
				Proxy:           http.ProxyURL(proxyURL),
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			client := &http.Client{
				Transport: transport,
				Timeout:   8 * time.Second,
			}

			req, err := http.NewRequestWithContext(ctx, "GET", "https://appbowl.com/api/sms/send-sms", nil)
			if err != nil {
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == 404 {
				mu.Lock()
				if foundProxy == "" {
					foundProxy = p
					cancel()
				}
				mu.Unlock()
			}
		}(proxy)
	}

	wg.Wait()
	return foundProxy
}

func getWorkingProxyWithRetry() string {
	for attempt := 1; attempt <= 2; attempt++ {
		proxies := fetchProxyList()
		if len(proxies) > 0 {
			working := testProxiesParallel(proxies)
			if working != "" {
				return working
			}
		}
	}
	return ""
}

func sendSMSRequest(proxy, number, baseURL string) map[string]interface{} {
	userName := gofakeit.Name()

	proxyURL, _ := url.Parse("http://" + proxy)
	transport := &http.Transport{
		Proxy:           http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	data := map[string]string{
		"name":   userName,
		"mobile": "+91" + number,
	}
	jsonData, _ := json.Marshal(data)

	req, _ := http.NewRequest("POST", "https://appbowl.com/api/sms/send-sms", bytes.NewBuffer(jsonData))

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-IN,en-GB;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://appbowl.com")
	req.Header.Set("Referer", "https://appbowl.com/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", gofakeit.UserAgent())
	req.Header.Set("sec-ch-ua", `"Chromium";v="137", "Not/A)Brand";v="24"`)
	req.Header.Set("sec-ch-ua-mobile", "?1")
	req.Header.Set("sec-ch-ua-platform", `"Android"`)
	req.Header.Set("Cookie", `connect.sid=s%3Axke1a96BgUsANKynFHUJeu0RGZDo1FKk.9MdJdsHcMXOZU9QL8XJG2Ht%2FxvZ4n9P%2Biyt%2BlA%2BCF8U; _ga_G5EZLNMVEW=GS2.1.s1772251637$o1$g0$t1772251637$j60$l0$h0; _ga=GA1.1.1248699123.1772251638; _hjSessionUser_5336592=eyJpZCI6IjYxYmFkNTdlLTE4MDMtNWUyMi04NGEwLTY4ODI2NjUzNTE0OSIsImNyZWF0ZWQiOjE3NzIyNTE2Mzc4MDAsImV4aXN0aW5nIjp0cnVlfQ==; _fbp=fb.1.1772251638664.668012148267289602`)

	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{
			"status":   "error",
			"number":   "+91" + number,
			"response": "Request failed",
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	bodyLower := strings.ToLower(bodyStr)

	if resp.StatusCode == 200 {
		var respJSON map[string]interface{}
		json.Unmarshal(body, &respJSON)

		otpSent := false
		if success, ok := respJSON["success"].(bool); ok && success {
			otpSent = true
		} else if strings.Contains(bodyLower, "otp") || strings.Contains(bodyLower, "sent") {
			otpSent = true
		} else if respMap, ok := respJSON["response"].(map[string]interface{}); ok {
			if respType, ok := respMap["type"].(string); ok && respType == "success" {
				otpSent = true
			}
		} else {
			otpSent = true
		}

		if otpSent {
			sessionID := storeSession(number, userName, proxy)
			return map[string]interface{}{
				"status":    "success",
				"number":    "+91" + number,
				"response":  "OTP Sent Successfully Via SMS.",
				"validity":  "10 Minutes",
				"verify_at": fmt.Sprintf("%s/verify?id=%s&otp=<OTP_code>", baseURL, sessionID),
			}
		}
	}

	return map[string]interface{}{
		"status":   "error",
		"number":   "+91" + number,
		"response": fmt.Sprintf("HTTP %d", resp.StatusCode),
	}
}

func callVerifyOTP(proxy, number, otp string) (int, map[string]interface{}, error) {
	proxyURL, _ := url.Parse("http://" + proxy)
	transport := &http.Transport{
		Proxy:           http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   3 * time.Second,
	}

	payload := map[string]string{
		"mobile": "+91" + number,
		"otp":    otp,
	}
	jsonData, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "https://appbowl.com/api/sms/verify-otp", bytes.NewBuffer(jsonData))

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-IN,en-GB;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://appbowl.com")
	req.Header.Set("Referer", "https://appbowl.com/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("sec-ch-ua", `"Chromium";v="137", "Not/A)Brand";v="24"`)
	req.Header.Set("sec-ch-ua-mobile", "?1")
	req.Header.Set("sec-ch-ua-platform", `"Android"`)
	req.Header.Set("Cookie", `connect.sid=s%3Axke1a96BgUsANKynFHUJeu0RGZDo1FKk.9MdJdsHcMXOZU9QL8XJG2Ht%2FxvZ4n9P%2Biyt%2BlA%2BCF8U; _ga_G5EZLNMVEW=GS2.1.s1772251637$o1$g0$t1772251637$j60$l0$h0; _ga=GA1.1.1248699123.1772251638; _hjSessionUser_5336592=eyJpZCI6IjYxYmFkNTdlLTE4MDMtNWUyMi04NGEwLTY4ODI2NjUzNTE0OSIsImNyZWF0ZWQiOjE3NzIyNTE2Mzc4MDAsImV4aXN0aW5nIjp0cnVlfQ==; _fbp=fb.1.1772251638664.668012148267289602`)
	req.Header.Set("User-Agent", gofakeit.UserAgent())

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var respJSON map[string]interface{}
	json.Unmarshal(body, &respJSON)

	return resp.StatusCode, respJSON, nil
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.GET("/send", func(c *gin.Context) {
		number := c.Query("number")

		if len(number) != 10 {
			c.IndentedJSON(http.StatusOK, gin.H{"status": "error", "message": "10-digit Number Required"})
			return
		}
		for _, ch := range number {
			if ch < '0' || ch > '9' {
				c.IndentedJSON(http.StatusOK, gin.H{"status": "error", "message": "10-digit Number Required"})
				return
			}
		}

		baseURL := getBaseURL(c)

		workingProxy := getWorkingProxyWithRetry()
		if workingProxy == "" {
			c.IndentedJSON(http.StatusOK, gin.H{"status": "error", "message": "No working proxy found"})
			return
		}

		smsResponse := sendSMSRequest(workingProxy, number, baseURL)

		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "    ")
		enc.Encode(smsResponse)
		c.Data(http.StatusOK, "application/json; charset=utf-8", buf.Bytes())
	})

	r.GET("/verify", func(c *gin.Context) {
		id := c.Query("id")
		otp := c.Query("otp")

		if id == "" || otp == "" {
			c.IndentedJSON(http.StatusOK, gin.H{"status": "error", "msg": "id and otp required"})
			return
		}

		meta := getSession(id)
		if meta == nil {
			c.IndentedJSON(http.StatusOK, gin.H{"status": "error", "msg": "ID Not Found."})
			return
		}

		number := meta.Number
		activeProxy := meta.Proxy

		status, respJSON, err := callVerifyOTP(activeProxy, number, otp)
		if err != nil {
			activeProxy = getWorkingProxyWithRetry()
			if activeProxy == "" {
				c.IndentedJSON(http.StatusOK, gin.H{
					"status": "error",
					"msg":    "Stored proxy failed and no replacement proxy found.",
					"number": "+91" + number,
				})
				return
			}

			status, respJSON, err = callVerifyOTP(activeProxy, number, otp)
			if err != nil {
				c.IndentedJSON(http.StatusOK, gin.H{
					"status": "error",
					"msg":    "Verification request failed even with new proxy.",
					"number": "+91" + number,
				})
				return
			}
		}

		if success, ok := respJSON["success"].(bool); ok && !success {
			c.IndentedJSON(http.StatusOK, gin.H{
				"status": "failed",
				"msg":    "Invalid OTP",
				"number": "+91" + number,
			})
			return
		}

		if success, ok := respJSON["success"].(bool); ok && success {
			cacheMu.Lock()
			delete(otpCache, id)
			delete(numberToID, number)
			cacheMu.Unlock()

			c.IndentedJSON(http.StatusOK, gin.H{
				"status": "success",
				"msg":    "OTP Verified Successfully",
				"number": "+91" + number,
			})
			return
		}

		c.IndentedJSON(http.StatusOK, gin.H{
			"status": "error",
			"msg":    fmt.Sprintf("Unexpected response: HTTP %d", status),
			"number": "+91" + number,
		})
	})

	r.Run(fmt.Sprintf("0.0.0.0:%d", PORT))
}
