package observability

import (
	"context"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)

func InitTracer(serviceName, jaegerURL string) (func(context.Context), error) {
	// Create a gRPC exporter that will send data to Jaeger
	_, err := otlptracegrpc.New(context.Background(), otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint(jaegerURL))
	if err != nil {
		return nil, err
	}

	// TODO: Добавить полную реализацию трейсинга
	// Пока возвращаем заглушку
	return func(ctx context.Context) {
		// Заглушка для shutdown функции
	}, nil
}
