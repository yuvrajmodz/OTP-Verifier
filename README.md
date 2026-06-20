# OTP-Verifier - SMS Verification API

A high-performance, robust API service written in Go for managing SMS OTP Generation and verification.

## Features

* **High Performance**: Built on top of the Fast and Minimal Framework.
* **In-Memory Sessions**: Maintains a Lightweight, thread-safe, in-memory cache for OTP sessions with automatic Time-To-Live (TTL) Cleanup.
* **Robust Error Handling & Retries**: Includes Built-in Mechanisms for Proxy Fallback and Request Retries.
* **Production Ready**: Optimized to run quietly and efficiently in production environments.

## Requirements

* [Go 1.21+](https://go.dev/dl/)

## Installation & Setup

1. **Clone or Download** The Repository to your Machine.
   ```bash
   git clone https://github.com/yuvrajmodz/OTP-Verifier.git
   ```
3. **Install Dependencies**:
   Open a Terminal in the Project Directory and Run:
   ```bash
   go mod tidy
   ```
4. **Run the Application**:
   Start the API server directly using:
   ```bash
   go run main.go
   ```
   *The server will start in Production Mode on `0.0.0.0:5002`.*

5. **Build for Production** (Optional):
   Compile the code into an executable for deployment:
   ```bash
   go build -o main
   ./main
   ```

## API Usage Guide

The API Exposes Two primary endpoints to handle the sending and verification of OTPs.

### 1. Send OTP
Initiates an OTP request to a given mobile number.

* **Endpoint**: `/send`
* **Method**: `GET`
* **Query Parameters**:
  * `number` (required): The 10-digit mobile number.

**Example Request:**
```bash
curl "http://example.com/send?number=9876543210"
```

**Example Response:**
```json
{
  "number": "+919876543210",
  "response": "OTP Sent Successfully Via SMS.",
  "status": "success",
  "validity": "10 Minutes",
  "verify_at": "http://example.com/verify?id=abcdefgh&otp=<OTP_code>"
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
curl "http://example.com/verify?id=abcdefgh&otp=123456"
```

**Example Response:**
```json
{
  "msg": "OTP Verified Successfully",
  "number": "+919876543210",
  "status": "success"
}
```

## About Developer

**Engineered By**: [Nactire](https://t.me/lieqy) && [NacDevs](https://t.me/NacDevs)
