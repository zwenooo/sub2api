package service

import "strings"

const (
	GatewaySchedulingStrategyBalanced         = "balanced"
	GatewaySchedulingStrategySingleExhaustion = "single_exhaustion"
)

func canonicalGatewaySchedulingStrategy(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case GatewaySchedulingStrategyBalanced:
		return GatewaySchedulingStrategyBalanced
	case GatewaySchedulingStrategySingleExhaustion:
		return GatewaySchedulingStrategySingleExhaustion
	default:
		return ""
	}
}

func NormalizeGatewaySchedulingStrategy(strategy string) string {
	if strings.TrimSpace(strategy) == "" {
		return GatewaySchedulingStrategyBalanced
	}
	if normalized := canonicalGatewaySchedulingStrategy(strategy); normalized != "" {
		return normalized
	}
	return GatewaySchedulingStrategyBalanced
}

func IsGatewaySchedulingStrategyValid(strategy string) bool {
	if strings.TrimSpace(strategy) == "" {
		return false
	}
	return canonicalGatewaySchedulingStrategy(strategy) != ""
}
