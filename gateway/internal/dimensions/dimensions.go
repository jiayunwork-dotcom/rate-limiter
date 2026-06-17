package dimensions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ratelimiter/gateway/pkg/models"
)

type Extractor struct{}

func NewExtractor() *Extractor {
	return &Extractor{}
}

func (e *Extractor) ExtractValue(dim models.Dimension, req *models.RequestContext) string {
	switch dim.Type {
	case models.DimensionAPI:
		return req.APIPath
	case models.DimensionMethod:
		return req.Method
	case models.DimensionUserID:
		return req.UserID
	case models.DimensionTenantID:
		return req.TenantID
	case models.DimensionIP:
		return req.ClientIP
	case models.DimensionHeader:
		if req.Headers != nil {
			return req.Headers[strings.ToLower(dim.HeaderName)]
		}
		return ""
	default:
		return ""
	}
}

func (e *Extractor) ExtractAll(dims models.RuleDimensions, req *models.RequestContext) map[string]string {
	result := make(map[string]string)
	for _, dim := range dims.Dimensions {
		result[string(dim.Type)] = e.ExtractValue(dim, req)
	}
	return result
}

func (e *Extractor) GenerateBucketKeys(rule *models.RuleConfig, req *models.RequestContext) []string {
	dims := rule.Dimensions
	if len(dims.Dimensions) == 0 {
		return []string{e.generateSingleKey(rule.ID, []string{"default"}, []string{"*"})}
	}

	if dims.CombineMode == "AND" {
		keys := make([]string, 0, len(dims.Dimensions))
		values := make([]string, 0, len(dims.Dimensions))
		for _, dim := range dims.Dimensions {
			keys = append(keys, string(dim.Type))
			val := e.ExtractValue(dim, req)
			if val == "" {
				val = "unknown"
			}
			values = append(values, val)
		}
		return []string{e.generateSingleKey(rule.ID, keys, values)}
	}

	bucketKeys := make([]string, 0, len(dims.Dimensions))
	for _, dim := range dims.Dimensions {
		val := e.ExtractValue(dim, req)
		if val == "" {
			val = "unknown"
		}
		bucketKeys = append(bucketKeys, e.generateSingleKey(
			rule.ID,
			[]string{string(dim.Type)},
			[]string{val},
		))
	}
	return bucketKeys
}

func (e *Extractor) generateSingleKey(ruleID string, keys []string, values []string) string {
	h := sha256.New()
	h.Write([]byte(ruleID))
	h.Write([]byte("|"))
	h.Write([]byte(strings.Join(keys, ",")))
	h.Write([]byte("|"))
	h.Write([]byte(strings.Join(values, "|")))
	sum := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("rl:%s:%s", ruleID, sum[:16])
}

func (e *Extractor) MatchesRule(rule *models.RuleConfig, req *models.RequestContext) bool {
	if rule.APIPath != "" && rule.APIPath != "*" {
		if !matchPath(rule.APIPath, req.APIPath) {
			return false
		}
	}
	if rule.Method != "" && rule.Method != "*" {
		if !strings.EqualFold(rule.Method, req.Method) {
			return false
		}
	}
	return true
}

func matchPath(pattern, path string) bool {
	if pattern == path {
		return true
	}
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i, part := range patternParts {
		if part == "*" {
			continue
		}
		if part != pathParts[i] {
			return false
		}
	}
	return true
}

type QuotaBucketKeyGenerator struct{}

func NewQuotaBucketKeyGenerator() *QuotaBucketKeyGenerator {
	return &QuotaBucketKeyGenerator{}
}

func (q *QuotaBucketKeyGenerator) Generate(level models.QuotaLevel, identifier string) string {
	return fmt.Sprintf("quota:%s:%s", level, identifier)
}

func (q *QuotaBucketKeyGenerator) GenerateForRequest(req *models.RequestContext) []struct {
	Level      models.QuotaLevel
	Identifier string
	Key        string
} {
	result := make([]struct {
		Level      models.QuotaLevel
		Identifier string
		Key        string
	}, 0, 4)

	result = append(result, struct {
		Level      models.QuotaLevel
		Identifier string
		Key        string
	}{
		Level:      models.QuotaLevelGlobal,
		Identifier: "global",
		Key:        q.Generate(models.QuotaLevelGlobal, "global"),
	})

	if req.TenantID != "" {
		result = append(result, struct {
			Level      models.QuotaLevel
			Identifier string
			Key        string
		}{
			Level:      models.QuotaLevelTenant,
			Identifier: req.TenantID,
			Key:        q.Generate(models.QuotaLevelTenant, req.TenantID),
		})
	}

	if req.UserID != "" {
		userIdent := fmt.Sprintf("%s:%s", req.TenantID, req.UserID)
		result = append(result, struct {
			Level      models.QuotaLevel
			Identifier string
			Key        string
		}{
			Level:      models.QuotaLevelUser,
			Identifier: userIdent,
			Key:        q.Generate(models.QuotaLevelUser, userIdent),
		})
	}

	if req.APIPath != "" {
		apiIdent := fmt.Sprintf("%s:%s", req.Method, req.APIPath)
		result = append(result, struct {
			Level      models.QuotaLevel
			Identifier string
			Key        string
		}{
			Level:      models.QuotaLevelAPI,
			Identifier: apiIdent,
			Key:        q.Generate(models.QuotaLevelAPI, apiIdent),
		})
	}

	return result
}
