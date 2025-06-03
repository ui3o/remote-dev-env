package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func getEnvString(env string, def string) string {
	if val, ok := os.LookupEnv(env); ok {
		return val
	}
	return def
}

func getEnvInt64(env string, def time.Duration) time.Duration {
	if val, ok := os.LookupEnv(env); ok {
		if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			return time.Duration(n)
		}
	}
	return def
}

func getOriginDir(addonName string) string {
	dir := strings.TrimSuffix(addonName, filepath.Ext(addonName))
	return filepath.Join(originDir, dir)
}

func getAddonDir(addonName string) string {
	dir := strings.TrimSuffix(addonName, filepath.Ext(addonName))
	return filepath.Join(addonDir, dir)
}

func getOriginPath(addonName string) string {
	dir := strings.TrimSuffix(addonName, filepath.Ext(addonName))
	return filepath.Join(originDir, dir, addonName)
}

func getAddonPath(addonName string) string {
	dir := strings.TrimSuffix(addonName, filepath.Ext(addonName))
	return filepath.Join(addonDir, dir, addonName)
}

func getDummyOriginConfigPath() string {
	return filepath.Join("dummy", addonConfigName)
}

type LogLevel int

const (
	EMERG LogLevel = iota
	ALERT
	CRIT
	ERR
	WARNING
	NOTICE
	INFO
	DEBUG
)

func (ll LogLevel) String() string {
	return [...]string{"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug"}[ll]
}
