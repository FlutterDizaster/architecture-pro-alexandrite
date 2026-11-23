// service-a: сервис заказов. На GET / спрашивает цену у service-b и отдаёт ответ.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

func initTracer(ctx context.Context, name string) (*sdktrace.TracerProvider, error) {
	// адрес коллектора берётся из OTEL_EXPORTER_OTLP_ENDPOINT
	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(name))),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tp, nil
}

func main() {
	ctx := context.Background()
	tp, err := initTracer(ctx, "service-a")
	if err != nil {
		log.Fatal(err)
	}
	defer tp.Shutdown(ctx)

	bURL := os.Getenv("SERVICE_B_URL")
	if bURL == "" {
		bURL = "http://service-b:8080"
	}
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport), Timeout: 5 * time.Second}

	handler := func(w http.ResponseWriter, r *http.Request) {
		orderID := r.URL.Query().Get("order_id")
		if orderID == "" {
			orderID = "demo-1"
		}
		trace.SpanFromContext(r.Context()).SetAttributes(attribute.String("order.id", orderID))

		req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, bURL+"/price?order_id="+orderID, nil)
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "service-b unavailable: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(w, "order %s: %s\n", orderID, body)
	}

	http.Handle("/", otelhttp.NewHandler(http.HandlerFunc(handler), "GET /"))
	log.Println("service-a on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
