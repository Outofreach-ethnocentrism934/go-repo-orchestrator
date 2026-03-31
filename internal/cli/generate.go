package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/agelxnash/go-repo-orchestrator/internal/config"
)

func newGenerateCommand(v *viper.Viper, logger *zap.Logger) *cobra.Command {
	if v == nil {
		v = viper.New()
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Сканирует текущую директорию и генерирует YAML-конфиг",
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath := strings.TrimSpace(v.GetString("config"))
			if err := validateConfigPath(configPath); err != nil {
				return err
			}

			absConfigPath, err := filepath.Abs(configPath)
			if err != nil {
				return fmt.Errorf("определить абсолютный путь конфига: %w", err)
			}

			configDir := filepath.Dir(absConfigPath)
			if err := os.MkdirAll(configDir, 0o755); err != nil {
				return fmt.Errorf("создать директорию конфига: %w", err)
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("получить текущую директорию: %w", err)
			}

			cfg, err := config.ScanDirectory(cmd.Context(), cwd)
			if err != nil {
				return fmt.Errorf("просканировать директорию: %w", err)
			}

			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("сериализовать конфиг: %w", err)
			}

			if err := os.WriteFile(absConfigPath, data, 0o644); err != nil {
				return fmt.Errorf("записать конфиг: %w", err)
			}

			logger.Info("successfully generated config", zap.String("path", absConfigPath), zap.Int("repos_found", len(cfg.Repos)))
			fmt.Printf("Поздравляю! YAML-конфиг успешно сгенерирован: %s\n", absConfigPath)
			fmt.Printf("Сканирование выполнено из директории: %s\n", cwd)
			fmt.Printf("Обнаружено репозиториев: %d\n", len(cfg.Repos))
			return nil
		},
	}

	return cmd
}
