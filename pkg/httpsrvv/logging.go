/*
Copyright 2025 Kubotal

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package httpsrv

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-logr/logr"
)

// responseWriter wraps http.ResponseWriter to capture response details
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
	size       int64
}

// newResponseWriter creates a new response writer wrapper
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     200, // default status code
		body:           new(bytes.Buffer),
	}
}

// Write captures the response body and calculates size
func (rw *responseWriter) Write(b []byte) (int, error) {
	// Write to the actual response
	n, err := rw.ResponseWriter.Write(b)

	// Capture for logging (limit size to prevent memory issues)
	if rw.body.Len() < 4096 { // limit to 4KB for logging
		rw.body.Write(b[:n])
	}

	rw.size += int64(n)
	return n, err
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware wraps an HTTP handler to log request and response details
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Get logger from context
		logger := logr.FromContextAsSlogLogger(r.Context())

		// Read and capture request body (if present)
		var requestBody []byte
		var requestBodySize int64
		if r.Body != nil {
			requestBody, _ = io.ReadAll(r.Body)
			requestBodySize = int64(len(requestBody))
			// Restore the body for the next handler
			r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// Log request information
		if logger != nil {
			logger.Debug("HTTP Request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("raw_query", r.URL.RawQuery),
				//slog.String("remote_addr", r.RemoteAddr),
				//slog.String("user_agent", r.UserAgent()),
				//slog.String("host", r.Host),
				//slog.String("referer", r.Referer()),
				//slog.Int64("content_length", r.ContentLength),
				slog.Int64("request_body_size", requestBodySize),
				//slog.String("content_type", r.Header.Get("Content-Type")),
				//slog.Any("request_headers", r.Header),
				slog.String("request_body", getSafeBodyString(requestBody)),
			)
		}

		// Wrap the response writer to capture response details
		rw := newResponseWriter(w)

		// Call the next handler
		next.ServeHTTP(rw, r)

		// Calculate duration
		duration := time.Since(start)

		// Log response information
		if logger != nil {
			logger.Debug("HTTP Response",
				//slog.String("method", r.Method),
				//slog.String("path", r.URL.Path),
				slog.Int("status_code", rw.statusCode),
				slog.Int64("response_size", rw.size),
				slog.Duration("duration", duration),
				//slog.String("duration_ms", formatDuration(duration)),
				slog.Any("response_headers", w.Header()),
				slog.String("response_body", getSafeBodyString(rw.body.Bytes())),
			)
		}
	})
}

// getSafeBodyString returns a safe string representation of the body
// Truncates long bodies and handles binary content
func getSafeBodyString(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	// Check if it's likely binary content
	if !isPrintable(body) {
		return "[binary content]"
	}

	// Truncate if too long
	const maxBodyLogSize = 1024 // 1KB
	if len(body) > maxBodyLogSize {
		return string(body[:maxBodyLogSize]) + "... [truncated]"
	}

	return string(body)
}

// isPrintable checks if the byte slice contains mostly printable characters
func isPrintable(data []byte) bool {
	if len(data) == 0 {
		return true
	}

	printableCount := 0
	for _, b := range data {
		if (b >= 32 && b <= 126) || b == '\n' || b == '\r' || b == '\t' {
			printableCount++
		}
	}

	// Consider it printable if at least 80% of characters are printable
	return float64(printableCount)/float64(len(data)) >= 0.8
}

// formatDuration formats duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return d.String()
	}
	if d < time.Millisecond {
		return d.Truncate(time.Microsecond).String()
	}
	if d < time.Second {
		return d.Truncate(time.Millisecond).String()
	}
	return d.Truncate(time.Millisecond).String()
}
