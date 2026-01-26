/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	containerTask "kettle/api/kettle"
	client "kettle/client"
	"log"
	"time"

	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: `start an existing runc container process`,
	Long: `Contacts kettle grpc server which inturn calls the respective ttrpc server to run start
  with the given process for a given existing container.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("start called")
		ctx := cmd.Context() // Get Cobra's context

		id, err := cmd.Flags().GetString("id")
		if err != nil {
			log.Fatalf("Failed to get id flag: %v", err)
		}
		if id == "" {
			log.Fatalf("Container ID is required")
		}

		clientContext, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		testConnection()
		client, err := client.GetGRPCTaskClient(clientContext)
		if err != nil {
			log.Fatalf("Failed to create task client: %v", err)
		}
		req := containerTask.StartRequest{
			ContainerId: id,
		}

		resp, _ := client.Start(clientContext, &req)
		if err != nil {
			log.Fatalf("Failed to get id flag: %v", err)
		}
		fmt.Println(resp)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	// Here you will define your flags and configuration settings.
	startCmd.PersistentFlags().String("id", "", "container id")

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// startCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// startCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
