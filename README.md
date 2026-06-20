# CronBot - SMS Verification API

A high-performance, robust API service written in Go for managing SMS OTP (One-Time Password) generation and verification.

## Features

* **High Performance**: Built on top of the fast and minimal [Gin Web Framework](https://gin-gonic.com/).
* **Proxy Management**: Automatically fetches, tests, and utilizes free proxies in parallel to bypass external API rate limits.
* **In-Memory Sessions**: Maintains a lightweight, thread-safe, in-memory cache for OTP sessions with automatic Time-To-Live (TTL) cleanup.
* **Robust Error Handling & Retries**: Includes built-in mechanisms for proxy fallback and request retries.
* **Production Ready**: Optimized to run quietly and efficiently in production environments.

## Requirements

* [Go 1.21+](https://go.dev/dl/)

## Installation & Setup

1. **Clone or Download** the repository to your local machine.
2. **Install Dependencies**:
   Open a terminal in the project directory and run:
   ```bash
   go mod tidy
   ```
3. **Run the Application**:
   Start the API server directly using:
   ```bash
   go run main.go
   ```
   *The server will start in Production Mode on `0.0.0.0:5002`.*

4. **Build for Production** (Optional):
   Compile the code into an executable for deployment:
   ```bash
   go build -o cronbot main.go
   ./cronbot
   ```

## API Usage Guide

The API exposes two primary endpoints to handle the sending and verification of OTPs.

### 1. Send OTP
Initiates an OTP request to a given mobile number.

* **Endpoint**: `/send`
* **Method**: `GET`
* **Query Parameters**:
  * `number` (required): The 10-digit mobile number.

**Example Request:**
```bash
curl "http://localhost:5002/send?number=9876543210"
```

**Example Response:**
```json
{
  "number": "+919876543210",
  "response": "OTP Sent Successfully Via SMS.",
  "status": "success",
  "validity": "10 Minutes",
  "verify_at": "http://localhost:5002/verify?id=abcdefgh&otp=<OTP_code>"
}
```

### 2. Verify OTP
Validates the OTP entered by the user against the active session.

* **Endpoint**: `/verify`
* **Method**: `GET`
* **Query Parameters**:
  * `id` (required): The session ID returned by the `/send` endpoint.
  * `otp` (required): The OTP code received by the user.

**Example Request:**
```bash
curl "http://localhost:5002/verify?id=abcdefgh&otp=123456"
```

**Example Response:**
```json
{
  "msg": "OTP Verified Successfully",
  "number": "+919876543210",
  "status": "success"
}
```

## How It Works Internally

1. **Proxy Rotation**: When a request comes in, the app pulls a fresh list of proxies from `proxyscrape.com`. It tests up to 15 of them concurrently and selects the first one that successfully reaches the upstream provider.
2. **Session Persistence**: Upon a successful SMS dispatch, a unique 8-character ID is generated and linked to the user's phone number. This session is securely cached in memory and automatically expires and deletes itself after 10 minutes.
3. **Verification**: During verification, the app seamlessly routes the request through the same active proxy used during dispatch to maintain session consistency. If the proxy dies, it attempts to fetch a replacement immediately.
