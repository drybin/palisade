package usecase

import "testing"

func TestPaperFillQuantity_respectsTopLevelLiquidity(t *testing.T) {
	if got := paperFillQuantity(10, 3); got != 3 {
		t.Fatalf("expected 3, got %.8f", got)
	}
	if got := paperFillQuantity(10, 0); got != 10 {
		t.Fatalf("expected full fill when liquidity is unknown, got %.8f", got)
	}
}

func TestPaperFillQuantity_doesNotExceedRemaining(t *testing.T) {
	if got := paperFillQuantity(2, 10); got != 2 {
		t.Fatalf("expected 2, got %.8f", got)
	}
}
