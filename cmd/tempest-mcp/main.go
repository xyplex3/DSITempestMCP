// Command tempest-mcp is an MCP server that connects Claude to the DSI Tempest drum machine.
// It runs on stdio and is registered in claude_desktop_config.json.
//
// Usage:
//
//	tempest-mcp [--config path] [--device name] [--channel 1-16] [--debug] [--list-ports]
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"tempest-mcp/internal/config"
	"tempest-mcp/internal/midi"
	"tempest-mcp/internal/server"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var (
		configPath string
		device     string
		channel    int
		debug      bool
		midiTrace  bool
		listPorts  bool
	)

	cmd := &cobra.Command{
		Use:   "tempest-mcp",
		Short: "MCP server for the DSI/Sequential Tempest drum machine",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle --list-ports before loading anything else
			if listPorts {
				return printPorts()
			}

			// Build logger
			var log *zap.Logger
			var err error
			if debug {
				log, err = zap.NewDevelopment()
			} else {
				log, err = zap.NewProduction()
			}
			if err != nil {
				return fmt.Errorf("creating logger: %w", err)
			}
			defer func() { _ = log.Sync() }()
			sugar := log.Sugar()

			// Load config
			if configPath == "" {
				configPath, err = config.DefaultPath()
				if err != nil {
					return fmt.Errorf("resolving default config path: %w", err)
				}
			}
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			// Apply CLI overrides
			if device != "" {
				cfg.MIDI.DeviceName = device
			}
			if channel >= 1 && channel <= 16 {
				cfg.MIDI.Channel = uint8(channel)
			}
			if debug {
				cfg.Log.Level = "debug"
			}
			if midiTrace {
				cfg.Log.MIDITrace = true
			}

			sugar.Infow("Starting tempest-mcp",
				"device", cfg.MIDI.DeviceName,
				"channel", cfg.MIDI.Channel,
				"library", cfg.Library.Path,
			)

			// Start the MCP server
			srv := server.New(cfg, sugar)
			return srv.ServeStdio()
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Config file path (default: ~/.config/tempest-mcp/config.yaml)")
	cmd.Flags().StringVar(&device, "device", "", "MIDI device name substring (overrides config)")
	cmd.Flags().IntVar(&channel, "channel", 0, "MIDI channel 1–16 (overrides config)")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")
	cmd.Flags().BoolVar(&midiTrace, "midi-trace", false, "Log every raw MIDI byte")
	cmd.Flags().BoolVar(&listPorts, "list-ports", false, "Print available MIDI ports and exit")

	return cmd
}

func printPorts() error {
	outs, ins, err := midi.ListPorts()
	if err != nil {
		return fmt.Errorf("listing ports: %w", err)
	}
	fmt.Println("MIDI Output ports:")
	for _, o := range outs {
		fmt.Println("  ", o)
	}
	fmt.Println("MIDI Input ports:")
	for _, i := range ins {
		fmt.Println("  ", i)
	}
	return nil
}
