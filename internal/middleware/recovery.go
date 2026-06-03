package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"hynor/banana-mw/internal/requtil"
)

var panicLogMutex sync.Mutex

func WritePanicLog(method, path, errorMsg string) {
	panicLogMutex.Lock()
	defer panicLogMutex.Unlock()

	f, err := os.OpenFile("panic.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	now := time.Now().Format("2006-01-02T15:04:05")
	panicInfo := fmt.Sprintf("[%s] PANIC: method=%s path=%s error=%s\n", now, method, path, errorMsg)
	_, _ = f.WriteString(panicInfo)
}

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &requtil.StatusCaptureWriter{ResponseWriter: w}
		defer func() {
			if recovered := recover(); recovered != nil {
				errorMsg := fmt.Sprintf("%v", recovered)
				slog.Error("panic recovered",
					"method", r.Method,
					"path", r.URL.Path,
					"error", errorMsg,
				)
				WritePanicLog(r.Method, r.URL.Path, errorMsg)
				if sw.Status() == 0 {
					http.Error(sw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}
		}()
		next.ServeHTTP(sw, r)
	})
}
