package hello

import "testing"

func TestGreetDefault(t *testing.T) {
	got := Greet("")
	want := "Hello, world!"
	if got != want {
		t.Errorf("Greet(\"\") = %q, want %q", got, want)
	}
}

func TestGreetWithName(t *testing.T) {
	got := Greet("渭哲")
	want := "Hello, 渭哲!"
	if got != want {
		t.Errorf("Greet(\"渭哲\") = %q, want %q", got, want)
	}
}
