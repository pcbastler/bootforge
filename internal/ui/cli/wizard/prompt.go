// Package wizard implements the interactive setup wizard for Bootforge.
// charmbracelet/huh is only imported in this file — all other wizard code
// uses the wrapper functions defined here.
package wizard

import (
	"github.com/charmbracelet/huh"
)

// Option represents a selectable choice.
type Option[T comparable] struct {
	Label string
	Value T
}

// Input prompts for a single text value.
// If *value is empty, defaultVal is used as the pre-filled value so the user
// can accept it with Enter instead of having to type it out.
func Input(title, defaultVal string, value *string, validate func(string) error) error {
	if *value == "" && defaultVal != "" {
		*value = defaultVal
	}
	field := huh.NewInput().
		Title(title).
		Value(value)
	if validate != nil {
		field.Validate(validate)
	}
	return huh.NewForm(huh.NewGroup(field)).Run()
}

// Confirm prompts for a yes/no decision.
func Confirm(title string, value *bool) error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Value(value),
		),
	).Run()
}

// Select prompts the user to pick one option from a list.
func Select[T comparable](title string, options []Option[T], value *T) error {
	opts := make([]huh.Option[T], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o.Label, o.Value)
	}
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[T]().
				Title(title).
				Options(opts...).
				Value(value),
		),
	).Run()
}

// MultiSelect prompts the user to pick one or more options.
func MultiSelect[T comparable](title string, options []Option[T], values *[]T) error {
	opts := make([]huh.Option[T], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o.Label, o.Value)
	}
	return huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[T]().
				Title(title).
				Options(opts...).
				Value(values),
		),
	).Run()
}

// Note displays an informational message.
func Note(title, description string) error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(title).
				Description(description),
		),
	).Run()
}
