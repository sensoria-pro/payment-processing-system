package main

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

func main() {
	var kafkaBrokers string
	var dlqTopic string

	var rootCmd = &cobra.Command{Use: "dlq-tool"}
	rootCmd.PersistentFlags().StringVar(&kafkaBrokers, "brokers", "localhost:9092", "Kafka broker addresses")
	rootCmd.PersistentFlags().StringVar(&dlqTopic, "dlq-topic", "transactions.created.dlq", "DLQ topic name")

	var viewCmd = &cobra.Command{
		Use:   "view",
		Short: "View messages in the DLQ",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Viewing messages from %s...\n", dlqTopic)
			//TODO: Здесь будет логика подключения к Kafka consumer и чтения сообщений
			// с выводом их offset, key и value.
		},
	}

	var retryCmd = &cobra.Command{
		Use:   "retry [offset]",
		Short: "Retry a message from the DLQ by its offset",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			offset := args[0]
			fmt.Printf("Retrying message with offset %s from %s...\n", offset, dlqTopic)
			//TODO: Здесь будет логика:
			// 1. Найти сообщение в DLQ по offset.
			// 2. Опубликовать его в основной рабочий топик (например, 'transactions.created').
			// 3. (Опционально) Удалить его из DLQ.
		},
	}
	
	rootCmd.AddCommand(viewCmd, retryCmd)
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}