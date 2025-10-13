package main

import (
	"testing"
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
			for i := 0; i < 100; i++ {
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
