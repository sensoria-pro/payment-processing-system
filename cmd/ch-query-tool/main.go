package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/spf13/cobra"
)

func main() {
	var dsn string

	var rootCmd = &cobra.Command{Use: "ch-query-tool"}
	rootCmd.PersistentFlags().StringVar(&dsn, "dsn", "clickhouse://localhost:9000/default", "ClickHouse DSN")

	// Command to get suspicious transactions
	var suspiciousCmd = &cobra.Command{
		Use:   "suspicious",
		Short: "Get suspicious transactions",
		Run: func(_ *cobra.Command, _ []string) {
			conn := connect(dsn)
			
			defer func() {
				if err := conn.Close(); err != nil {
					log.Fatalf("Не удалось закрыть ClickHouse connection: %v", err)
				}
			}()

			query := "SELECT transaction_id, reason, processed_at FROM fraud_reports WHERE is_fraudulent = 1 ORDER BY processed_at DESC LIMIT 20"
			rows, err := conn.Query(context.Background(), query)
			if err != nil {
				log.Fatalf("Query failed: %v", err)
			}
			
			defer func() {
				if err := rows.Close(); err != nil{
					log.Fatalf("Не удалось закрыть: %v", err)
				}
			}()

			rows, err = conn.Query(context.Background(), "SELECT transaction_id, reason, processed_at FROM fraud_reports WHERE is_fraudulent = 1 ORDER BY processed_at DESC LIMIT 20")
			if err != nil {
				log.Fatal(err)
			}
			defer func() {
				if err := rows.Close(); err != nil{
					log.Fatalf("Не удалось закрыть: %v", err)
				}
			}()

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			
			if _, err := fmt.Fprintln(w, "TRANSACTION ID\tREASON\tPROCESSED AT"); err != nil {
				log.Fatalf("Не удалось закрыть writer: %v", err)
			}
			for rows.Next() {
				var id, reason string
				var processedAt time.Time
				if err := rows.Scan(&id, &reason, &processedAt); err != nil {
					log.Fatal(err)
				}
				

				if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", id, reason, processedAt.Format(time.RFC3339)); err != nil {
					log.Fatalf("Не удалось закрыть writer %v", err)
				}
			}
			
			if err := w.Flush(); err != nil {
				log.Fatalf("Не удалось закрыть writer: %v", err)
			}
			
		},
	}

	// Command to get top cards by number of transactions
	var topCardsCmd = &cobra.Command{
		Use:   "top-cards",
		Short: "Get top cards by transaction count",
		Run: func(cmd *cobra.Command, _ []string) {
			// Get the value of the --limit flag
			limit, _ := cmd.Flags().GetInt("limit")

			conn := connect(dsn)

			defer func() {
				if err := conn.Close(); err != nil {
					log.Fatalf("Не удалось закрыть ClickHouse connection: %v", err)
				}
			}()

			// Forming a SQL query for data aggregation
			query := "SELECT card_hash, count(*) AS total FROM fraud_reports GROUP BY card_hash ORDER BY total DESC LIMIT ?"
			rows, err := conn.Query(context.Background(), query, limit)
			if err != nil {
				log.Fatalf("Query failed: %v", err)
			}

			// Using tabwriter for beautiful tabular output
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			
			if _, err := fmt.Fprintln(w, "CARD HASH\tTRANSACTION COUNT"); err != nil {
				log.Fatalf("Не удалось закрыть writer %v", err)
			}

			for rows.Next() {
				var cardHash string
				var total uint64
				if err := rows.Scan(&cardHash, &total); err != nil {
					log.Fatal(err)
				}
				
				if _, err := fmt.Fprintf(w, "%s\t%d\n", cardHash, total); err != nil {
					log.Fatalf("Не удалось закрыть writer %v", err)
				}
			}
			if err := w.Flush(); err != nil {
				log.Fatalf("Не удалось закрыть writer: %v", err)
			}
		},
	}
	// Add the --limit flag to the top-cards command with a default value of 10
	topCardsCmd.Flags().Int("limit", 10, "Number of top cards to show")
	//TODO: Логика для top-cards...

	rootCmd.AddCommand(suspiciousCmd, topCardsCmd)
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Ошибка выполнения команды: %v", err)
	}
}

func connect(dsn string) clickhouse.Conn {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{dsn},

		// Adding a timeout to improve reliability
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("Не удалось подключиться к ClickHouse: %v", err)
	}
	return conn
}
