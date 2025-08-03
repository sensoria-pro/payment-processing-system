package main

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/spf13/cobra"
	"context"
)

func main() {
	var dsn string

	var rootCmd = &cobra.Command{Use: "ch-query-tool"}
	rootCmd.PersistentFlags().StringVar(&dsn, "dsn", "clickhouse://localhost:9000/default", "ClickHouse DSN")

	// Command to get suspicious transactions
	var suspiciousCmd = &cobra.Command{
		Use:   "suspicious",
		Short: "Get suspicious transactions",
		Run: func(cmd *cobra.Command, args []string) {
			conn := connect(dsn)
			defer conn.Close()

			rows, err := conn.Query(context.Background(), "SELECT transaction_id, reason, processed_at FROM fraud_reports WHERE is_fraudulent = 1 ORDER BY processed_at DESC LIMIT 20")
			if err != nil {
				log.Fatal(err)
			}
			
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "TRANSACTION ID\tREASON\tPROCESSED AT")
			for rows.Next() {
				var id, reason string
				var processedAt time.Time
				if err := rows.Scan(&id, &reason, &processedAt); err != nil {
					log.Fatal(err)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", id, reason, processedAt.Format(time.RFC3339))
			}
			w.Flush()
		},
	}
	
	// Command to get top cards by number of transactions
	var topCardsCmd = &cobra.Command{
		Use:   "top-cards",
		Short: "Get top cards by transaction count",
		Run: func(cmd *cobra.Command, args []string) {
			//TODO: Логика для top-cards...
		},
	}

	rootCmd.AddCommand(suspiciousCmd, topCardsCmd)
	rootCmd.Execute()
}

func connect(dsn string) clickhouse.Conn {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{dsn},
	})
	if err != nil {
		log.Fatal(err)
	}
	return conn
}