// app-ejemplo: demuestra el patrón cache-aside contra Valkey vía Sentinel.
//
// Endpoints:
//
//	GET /usuario/{id}  -> busca en caché; si falla (miss) simula una BD lenta
//	                      (50ms), guarda en caché con TTL y responde.
//	                      Header X-Cache: hit|miss ; body JSON con latencia.
//	GET /healthz       -> 200 si PING a Valkey responde.
//
// Modo sonda (para 99-verify.yml dentro de un pod sin shell):
//
//	app-ejemplo -probe http://127.0.0.1:8080/usuario/1
//	imprime por stdout: {"status":200,"cache":"miss","latency_ms":51}
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const cacheTTL = 60 * time.Second

func main() {
	// --- Modo sonda: actúa como cliente HTTP y sale (no levanta servidor). ---
	probeURL := flag.String("probe", "", "si se indica, hace GET a esa URL y emite JSON con status/cache/latencia")
	flag.Parse()
	if *probeURL != "" {
		runProbe(*probeURL)
		return
	}

	addr := getenv("HTTP_ADDR", ":8080")
	rdb := newClient()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /usuario/{id}", usuarioHandler(rdb))
	mux.HandleFunc("GET /healthz", healthHandler(rdb))

	log.Printf("escuchando en %s; sentinel=%s master=%s",
		addr, getenv("VALKEY_SENTINEL_ADDR", ""), getenv("VALKEY_MASTER_NAME", "mymaster"))
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

// newClient construye un FailoverClient que descubre el master vía Sentinel.
func newClient() *redis.Client {
	pwd := os.Getenv("VALKEY_PASSWORD")
	return redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:    getenv("VALKEY_MASTER_NAME", "mymaster"),
		SentinelAddrs: []string{getenv("VALKEY_SENTINEL_ADDR", "valkey.data.svc.cluster.local:26379")},
		Password:      pwd,
		// Bitnami protege también el puerto de Sentinel con el mismo password.
		SentinelPassword: pwd,
		DB:               0,
		DialTimeout:      3 * time.Second,
		ReadTimeout:      2 * time.Second,
		WriteTimeout:     2 * time.Second,
	})
}

type respuesta struct {
	Status    int    `json:"status"`
	Cache     string `json:"cache"`      // "hit" | "miss"
	LatencyMs int64  `json:"latency_ms"` // latencia observada en el handler
	Data      string `json:"data,omitempty"`
}

func usuarioHandler(rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		key := "usuario:" + id
		ctx := r.Context()
		start := time.Now()

		val, err := rdb.Get(ctx, key).Result()
		switch {
		case err == redis.Nil:
			// MISS: simulamos una consulta lenta a la BD.
			time.Sleep(50 * time.Millisecond)
			val = fmt.Sprintf(`{"id":"%s","nombre":"Usuario %s"}`, id, id)
			if setErr := rdb.Set(ctx, key, val, cacheTTL).Err(); setErr != nil {
				log.Printf("warn: no se pudo cachear %s: %v", key, setErr)
			}
			writeJSON(w, "miss", val, time.Since(start))
		case err != nil:
			// Error real hablando con Valkey.
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
		default:
			// HIT.
			writeJSON(w, "hit", val, time.Since(start))
		}
	}
}

func writeJSON(w http.ResponseWriter, cache, data string, elapsed time.Duration) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", cache)
	w.Header().Set("X-Latency-Ms", fmt.Sprintf("%d", elapsed.Milliseconds()))
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(respuesta{
		Status:    http.StatusOK,
		Cache:     cache,
		LatencyMs: elapsed.Milliseconds(),
		Data:      data,
	})
}

func healthHandler(rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := rdb.Ping(ctx).Err(); err != nil {
			http.Error(w, "valkey no responde", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}
}

// runProbe hace un GET y emite JSON {status,cache,latency_ms} por stdout.
// La latencia medida (round-trip) basta para distinguir MISS (~50ms) de HIT.
func runProbe(url string) {
	start := time.Now()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf(`{"status":0,"cache":"error","latency_ms":0,"error":%q}`+"\n", err.Error())
		os.Exit(1)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	elapsed := time.Since(start).Milliseconds()
	cache := resp.Header.Get("X-Cache")
	if cache == "" {
		cache = "unknown"
	}
	out := respuesta{Status: resp.StatusCode, Cache: cache, LatencyMs: elapsed}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
