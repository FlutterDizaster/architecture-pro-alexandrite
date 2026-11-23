// service-b: сервис расчёта стоимости. На GET /price отдаёт цену заказа.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func initTracer(ctx context.Context, name string) (*sdktrace.TracerProvider, error) {
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
	tp, err := initTracer(ctx, "service-b")
	if err != nil {
		log.Fatal(err)
	}
	defer tp.Shutdown(ctx)
	tracer := otel.Tracer("service-b")

	handler := func(w http.ResponseWriter, r *http.Request) {
		orderID := r.URL.Query().Get("order_id")
		// отдельный span на сам расчёт, чтобы в Jaeger было видно, сколько он занял
		_, span := tracer.Start(r.Context(), "calculate price")
		span.SetAttributes(attribute.String("order.id", orderID))
		time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)
		price := 10000 + rand.Intn(90000)
		span.SetAttributes(attribute.Int("order.price", price))
		span.End()
		fmt.Fprintf(w, "price %d RUB", price)
	}

	http.Handle("/price", otelhttp.NewHandler(http.HandlerFunc(handler), "GET /price"))
	log.Println("service-b on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
