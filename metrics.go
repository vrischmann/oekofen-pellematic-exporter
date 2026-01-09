package main

import (
	"fmt"
	"strings"
)

func cleanLabelName(name string) string {
	result := strings.ToLower(name)

	result = strings.TrimPrefix(result, "l_")

	result = strings.ReplaceAll(result, "_", "_")

	return result
}

func buildMetricName(prefix, componentName, field string) string {
	cleanField := cleanLabelName(field)

	result := fmt.Sprintf("pellematic_%s_%s", prefix, cleanField)

	return result
}
