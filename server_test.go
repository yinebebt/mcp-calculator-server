package main

import (
	"context"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEvaluateExpression(t *testing.T) {
	tests := []struct {
		name      string
		expr      string
		expected  float64
		shouldErr bool
	}{
		// Basic functionality
		{"addition", "2+3", 5, false},
		{"multiplication", "3*4", 12, false},
		{"precedence", "2+3*4", 14, false},
		{"parentheses", "(2+3)*4", 20, false},
		{"scientific", "1e2", 100, false},
		{"unary_minus", "-5+3", -2, false},

		// Error cases
		{"empty_parentheses", "()", 0, true},
		{"unmatched_closing", "1+2)", 0, true},
		{"trailing_operator", "2+", 0, true},
		{"division_by_zero", "5/0", 0, true},
		{"invalid_character", "2&3", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluateExpression(tt.expr)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error for expression %q, but got result: %v", tt.expr, result)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for expression %q: %v", tt.expr, err)
				return
			}

			// Use a small tolerance for floating point comparison
			const tolerance = 1e-10
			if abs(result-tt.expected) > tolerance {
				t.Errorf("expression %q: expected %v, got %v", tt.expr, tt.expected, result)
			}
		})
	}
}

// Helper function for floating point comparison
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// TestGenerateUniformExtremeRanges verifies huge ranges no longer overflow
// the int64 bound (which previously made rand.Int panic).
func TestGenerateUniformExtremeRanges(t *testing.T) {
	tests := []struct {
		name string
		min  float64
		max  float64
	}{
		{"huge_range", -1e300, 1e300},
		{"large_max", 0, 1e18},
		{"tiny_range", 0, 1e-8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := generateUniform(tt.min, tt.max)
			if err != nil {
				t.Fatalf("generateUniform(%g, %g) failed: %v", tt.min, tt.max, err)
			}
			if math.IsNaN(result) || math.IsInf(result, 0) {
				t.Errorf("generateUniform(%g, %g) returned non-finite value: %v", tt.min, tt.max, result)
			}
			if result < tt.min || result >= tt.max {
				t.Errorf("generateUniform(%g, %g) returned %v outside [min, max)", tt.min, tt.max, result)
			}
		})
	}
}

// TestGenerateNormalFinite verifies normal generation never yields NaN/Inf
// (log(0) in the Box-Muller transform previously could).
func TestGenerateNormalFinite(t *testing.T) {
	for range 1000 {
		result, err := generateNormal(0, 1)
		if err != nil {
			t.Fatalf("generateNormal failed: %v", err)
		}
		if math.IsNaN(result) || math.IsInf(result, 0) {
			t.Fatalf("generateNormal returned non-finite value: %v", result)
		}
	}
}

// TestHandleCalculateNonFinite verifies overflow results are reported as
// errors instead of "+Inf" successes.
func TestHandleCalculateNonFinite(t *testing.T) {
	res, out, err := handleCalculate(context.Background(), &mcp.CallToolRequest{}, calculateInput{
		Expression: "1e308 * 10",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Errorf("expected IsError result for overflowing expression, got result %q", out.Result)
	}
}

func TestHandleCompletion(t *testing.T) {
	tests := []struct {
		name     string
		ref      *mcp.CompleteReference
		argName  string
		argValue string
		expected []string
	}{
		{
			name:     "prompt_difficulty_prefix",
			ref:      &mcp.CompleteReference{Type: "ref/prompt", Name: "math_problem"},
			argName:  "difficulty",
			argValue: "h",
			expected: []string{"hard"},
		},
		{
			name:     "prompt_topic_all",
			ref:      &mcp.CompleteReference{Type: "ref/prompt", Name: "math_problem"},
			argName:  "topic",
			argValue: "",
			expected: []string{"addition", "subtraction", "multiplication", "division", "mixed"},
		},
		{
			name:     "resource_constant_prefix",
			ref:      &mcp.CompleteReference{Type: "ref/resource", URI: "math://constants/{name}"},
			argName:  "name",
			argValue: "ln",
			expected: []string{"ln2", "ln10"},
		},
		{
			name:     "unknown_prompt",
			ref:      &mcp.CompleteReference{Type: "ref/prompt", Name: "unknown"},
			argName:  "difficulty",
			argValue: "h",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := handleCompletion(context.Background(), &mcp.CompleteRequest{
				Params: &mcp.CompleteParams{
					Ref:      tt.ref,
					Argument: mcp.CompleteParamsArgument{Name: tt.argName, Value: tt.argValue},
				},
			})
			if err != nil {
				t.Fatalf("handleCompletion failed: %v", err)
			}
			want := slices.Clone(tt.expected)
			slices.Sort(want)
			if !slices.Equal(res.Completion.Values, want) {
				t.Errorf("expected sorted completions %v, got %v", want, res.Completion.Values)
			}
		})
	}
}

func TestHandleMathConstant(t *testing.T) {
	res, err := handleMathConstant(context.Background(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "math://constants/pi"},
	})
	if err != nil {
		t.Fatalf("handleMathConstant failed: %v", err)
	}
	if len(res.Contents) != 1 || !strings.Contains(res.Contents[0].Text, "3.14") {
		t.Errorf("unexpected contents: %+v", res.Contents)
	}

	_, err = handleMathConstant(context.Background(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "math://constants/nope"},
	})
	if err == nil {
		t.Error("expected error for unknown constant")
	}
}

// TestDistributionGenerators tests the three distribution generators
func TestDistributionGenerators(t *testing.T) {
	tests := []struct {
		name    string
		genFunc func(float64, float64) (float64, error)
		min     float64
		max     float64
	}{
		{"uniform", generateUniform, 0.0, 100.0},
		{"normal", generateNormal, 0.0, 100.0},
		{"exponential", generateExponential, 0.0, 100.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate 100 samples to verify they're within range
			for range 100 {
				result, err := tt.genFunc(tt.min, tt.max)
				if err != nil {
					t.Fatalf("Generator failed: %v", err)
				}
				if result < tt.min || result > tt.max {
					t.Errorf("Result %.2f outside range [%.2f, %.2f]",
						result, tt.min, tt.max)
				}
			}
		})
	}
}
