package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/spf13/cobra"
)

func main() {
	var kafkaBrokers string
	var dlqTopic string

	var rootCmd = &cobra.Command{Use: "dlq-tool"}
	rootCmd.PersistentFlags().StringVar(&kafkaBrokers, "brokers", "localhost:9092", "Адреса брокеров Kafka")
	rootCmd.PersistentFlags().StringVar(&dlqTopic, "dlq-topic", "transactions.created.dlq", "Имя DLQ топика")

	var viewCmd = &cobra.Command{
		Use:   "view",
		Short: "Просмотреть сообщения в DLQ",
		Run: func(cmd *cobra.Command, _ []string) {
			limit, _ := cmd.Flags().GetInt("limit")
			fmt.Printf("Просмотр последних %d сообщений из %s...\n", limit, dlqTopic)

			c, err := kafka.NewConsumer(&kafka.ConfigMap{
				"bootstrap.servers": kafkaBrokers,
				"group.id":          "dlq-tool-viewer",
				"auto.offset.reset": "earliest",
			})
			if err != nil {
				log.Fatalf("Не удалось создать consumer: %v", err)
			}
			defer func() {
				if err := c.Close(); err != nil {
					log.Fatalf("Не удалось закрыть consumer: %v", err)
				}
			}()

			if err := c.SubscribeTopics([]string{dlqTopic}, nil); err != nil {
				log.Fatalf("Ошибка подписки на топик: %v", err)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "OFFSET\tKEY\tERROR_TYPE\tERROR_STRING")
			fmt.Fprintln(w, "------\t---\t----------\t------------")

			msgCount := 0
			for msgCount < limit {
				msg, err := c.ReadMessage(10 * time.Second)
				if err != nil {
					if e, ok := err.(kafka.Error); ok && e.Code() == kafka.ErrTimedOut {
						break // final topic if no new messages
					}
					log.Printf("Ошибка consumer: %v\n", err)
					break
				}
				errorType, errorString := getErrorHeaders(msg.Headers)
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", msg.TopicPartition, string(msg.Key), errorType, errorString)
				msgCount++
			}
			w.Flush()
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

			fmt.Printf("Повторная отправка сообщения из %s (партиция %d, offset %d) в топик %s...\n", dlqTopic, partition, offset, targetTopic)

			// Consumer for reading a specific message
			c, err := kafka.NewConsumer(&kafka.ConfigMap{
				"bootstrap.servers": kafkaBrokers,
				"group.id":          "dlq-tool-retrier",
				"auto.offset.reset": "earliest",
			})
			if err != nil {
				log.Fatalf("Не удалось создать consumer: %v", err)
			}

			defer func() {
				if err := c.Close(); err != nil {
					log.Fatalf("Не удалось закрыть consumer: %v", err)
				}
			}()

			// Producer for sending message back to the main topic
			p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": kafkaBrokers})
			if err != nil {
				log.Fatalf("Не удалось создать producer: %v", err)
			}
			defer p.Close()

			// specify the specific partition and offset
			tp := kafka.TopicPartition{Topic: &dlqTopic, Partition: int32(partition), Offset: kafka.Offset(offset)}
			c.Assign([]kafka.TopicPartition{tp})

			// Read exactly one message
			msg, err := c.ReadMessage(5 * time.Second)
			if err != nil {
				log.Fatalf("Не удалось прочитать сообщение по указанному offset: %v", err)
			}

			// Publish it to the target topic
			retryMsg := &kafka.Message{
				TopicPartition: kafka.TopicPartition{Topic: &targetTopic, Partition: kafka.PartitionAny},
				Value:          msg.Value,
				Key:            msg.Key,
			}
			p.Produce(retryMsg, nil)
			p.Flush(5000)
			fmt.Println("Сообщение успешно отправлено на повторную обработку.")
		},
	}
	retryCmd.Flags().String("target-topic", "transactions.created", "Топик для повторной отправки сообщения")

	rootCmd.AddCommand(viewCmd, retryCmd)
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// Helper functions

// getErrorHeaders extracts error_type and error_string from Kafka headers
func getErrorHeaders(headers []kafka.Header) (string, string) {
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

// parsePartitionOffset parses "partition:offset" string into integers
func parsePartitionOffset(arg string) (int, int64) {
	parts := strings.Split(arg, ":")
	if len(parts) != 2 {
		log.Fatalf("Неверный формат. Ожидается partition:offset, например, 0:123")
	}
	partition, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Fatalf("Неверный номер партиции: %v", err)
	}
	offset, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		log.Fatalf("Неверный номер offset: %v", err)
	}
	return partition, offset
}
