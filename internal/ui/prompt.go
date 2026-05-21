package ui

import "github.com/charmbracelet/huh"

// Confirm shows a yes/no prompt and returns the user's choice.
func Confirm(title, affirmative, negative string) (bool, error) {
	var v bool
	err := huh.NewConfirm().
		Title(title).
		Affirmative(affirmative).
		Negative(negative).
		Value(&v).Run()
	return v, err
}

// Input shows a single-line text prompt with an optional validator.
func Input(title string, value *string, validate func(string) error) error {
	in := huh.NewInput().Title(title).Value(value)
	if validate != nil {
		in = in.Validate(validate)
	}
	return in.Run()
}

// Select shows a single-choice picker.
func Select[T comparable](title string, value *T, options []huh.Option[T]) error {
	return huh.NewSelect[T]().Title(title).Options(options...).Value(value).Run()
}
