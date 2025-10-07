package main

import (
	"context"
	"fmt"

	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/twmb/franz-go/pkg/kgo"

	"payment-processing-system/internal/config"
	"payment-processing-system/internal/observability"
)

func main() {
	// --- Configuration Setup ---
	cfg, err := config.Load("configs/config.yaml")
	
	logger := observability.SetupLogger(cfg.App.Env)
	logger.Info("DLQ service запускается", "env", cfg.App.Env)
	
	if err != nil {
		logger.Error("Failed to load config", "ERROR", err)
		os.Exit(1)
	}
	// --- Component Initialization ---
	var kafkaBrokers string
	var dlqTopic string

	//logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	var rootCmd = &cobra.Command{Use: "dlq-tool"}
	rootCmd.PersistentFlags().StringVar(&kafkaBrokers, "brokers", "localhost:9092", "Адреса брокеров Kafka")
	rootCmd.PersistentFlags().StringVar(&dlqTopic, "dlq-topic", "transactions.created.dlq", "Имя DLQ топика")

	var viewCmd = &cobra.Command{
		Use:   "view",
		Short: "Просмотреть сообщения в DLQ",
		Run: func(cmd *cobra.Command, _ []string) {
			limit, _ := cmd.Flags().GetInt("limit")
			logger.Info("просмотр последних сообщений", "topic", dlqTopic, "limit", limit)

			brokers := strings.Split(kafkaBrokers, ",")
			client, err := kgo.NewClient(
				kgo.SeedBrokers(brokers...),
				kgo.ConsumerGroup("dlq-tool-viewer"),
				kgo.ConsumeTopics(dlqTopic),
				kgo.FetchMaxWait(5*time.Second),
				// Начинаем читать с самого начала топика
				kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
			)
			if err != nil {
				logger.Error("не удалось создать consumer", "ERROR", err)
				os.Exit(1)
			}
			defer client.Close()

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			
			if _, err := fmt.Fprintln(w, "OFFSET\tKEY\tERROR_TYPE\tERROR_STRING"); err != nil {
				logger.Error("Не удалось закрыть writer", "ERROR", err)
			}
			
			if _, err := fmt.Fprintln(w, "------\t---\t----------\t------------"); err != nil {
				logger.Error("Не удалось закрыть writer", "ERROR", err)
			}

			msgCount := 0
			ctx := context.Background()

			for msgCount < limit {
				fetches := client.PollFetches(ctx)
				if fetches.IsClientClosed() {
					break
				}
				if len(fetches.Records()) == 0 {
					logger.Info("больше нет сообщений в топике")
					break
				}

				fetches.EachRecord(func(record *kgo.Record) {
					if msgCount >= limit {
						return
					}
					errorType, errorString := getErrorHeaders(record.Headers)
					fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", record.Offset, string(record.Key), errorType, errorString)
					msgCount++
				})
			}
			if err := w.Flush(); err != nil {
				logger.Error("Не удалось закрыть writer", "ERROR", err)
			}
		},
	}
	viewCmd.Flags().Int("limit", 10, "Количество сообщений для просмотра")

	var retryCmd = &cobra.Command{
		Use:   "retry [partition:offset]",
		Short: "Повторно отправить сообщение из DLQ по его партиции и offset",
		Args:  cobra.ExactArgs(1),
		
		Run: func(cmd *cobra.Command, args []string) {
			targetTopic, _ := cmd.Flags().GetString("target-topic")
			partition, offset := parsePartitionOffset(args[0])
			logger.Info("повторная отправка сообщения", "from_topic", dlqTopic, "partition", partition, "offset", offset, "to_topic", targetTopic)

			brokers := strings.Split(kafkaBrokers, ",")
			// Producer для отправки сообщения
			producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
			if err != nil {
				logger.Error("не удалось создать producer", "ERROR", err)
				os.Exit(1)
			}
			defer producer.Close()

			// Consumer для чтения одного конкретного сообщения
			consumer, err := kgo.NewClient(
				kgo.SeedBrokers(brokers...),
				kgo.ConsumerGroup("dlq-tool-retrier"),
				kgo.ConsumeTopics(dlqTopic),
				kgo.ConsumePartitions(map[string]map[int32]kgo.Offset{
					dlqTopic: {int32(partition): kgo.NewOffset().At(offset)},
				}),
			)
			if err != nil {
				logger.Error("не удалось создать consumer", "ERROR", err)
				os.Exit(1)
			}
			defer consumer.Close()

			logger.Info("чтение сообщения по указанному offset...")
			fetches := consumer.PollFetches(context.Background())
			if err := fetches.Err(); err != nil {
				logger.Error("не удалось прочитать сообщение", "ERROR", err)
				os.Exit(1)
			}
			record := fetches.Records()[0]
			if record == nil {
				logger.Error("сообщение по указанному offset не найдено")
				os.Exit(1)
			}
			
			retryRecord := &kgo.Record{
				Topic: targetTopic,
				Value: record.Value,
				Key:   record.Key,
			}
			// Отправляем синхронно, чтобы дождаться результата
			if err := producer.ProduceSync(context.Background(), retryRecord).FirstErr(); err != nil {
				logger.Error("не удалось повторно отправить сообщение", "ERROR", err)
				os.Exit(1)
			}

			logger.Info("сообщение успешно отправлено на повторную обработку")
		},
	}
	retryCmd.Flags().String("target-topic", "transactions.created", "Топик для повторной отправки сообщения")

	rootCmd.AddCommand(viewCmd, retryCmd)
	if err := rootCmd.Execute(); err != nil {
		logger.Error("ошибка выполнения команды", "ERROR", err)
		os.Exit(1)
	}
}

// Helper functions

// getErrorHeaders extracts error_type and error_string from Kafka headers
func getErrorHeaders(headers []kgo.RecordHeader) (string, string) {
	var errorType, errorString = "N/A", "N/A"
	for _, h := range headers {
		if h.Key == "error_type" {
			errorType = string(h.Value)
		}
		if h.Key == "error_string" {
			errorString = string(h.Value)
		}
	}
	return errorType, errorString
}

// parsePartitionOffset парсит строку "partition:offset" в целые числа
func parsePartitionOffset(arg string) (int, int64) {
	parts := strings.Split(arg, ":")
	if len(parts) != 2 {
		fmt.Errorf("Неверный формат. Ожидается partition:offset, например, 0:123")
	}
	partition, err := strconv.Atoi(parts[0])
	if err != nil {
		fmt.Errorf("Неверный номер партиции: %v", err)
	}
	offset, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		fmt.Errorf("Неверный номер offset: %v", err)
	}
	return partition, offset
}
