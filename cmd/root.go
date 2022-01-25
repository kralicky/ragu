package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kralicky/ragu/pkg/ragu"
	"github.com/spf13/cobra"
)

var outDir string

var rootCmd = &cobra.Command{
	Use:   "ragu [proto files...]",
	Short: "Generate Go protobuf code without protoc",
	Example: `$ ragu has_services.proto               
  # generates ./has_services.pb.go, ./has_services_grpc.pb.go
$ ragu -o pkg/types has_services.proto  
  # generates ./pkg/types/has_services.pb.go, ./pkg/types/has_services_grpc.pb.go
$ ragu -o pkg/types no_services.proto no_services_2.proto
  # generates ./pkg/types/no_services.pb.go ./pkg/types/no_services_2.pb.go`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, arg := range args {
			files, err := ragu.GenerateCode(arg)
			if err != nil {
				return err
			}
			for _, file := range files {
				// Write file to configured output directory
				path := filepath.Join(outDir, file.GetName())
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(path, []byte(file.GetContent()), 0644); err != nil {
					return err
				}
			}
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&outDir, "output", "o", ".", "output directory")
}
