//go:build ignore

package main

import (
	"fmt"
	"github.com/soar/inputview/internal/gpvskin"
)

func p(label string, props map[string]string) {
	fmt.Printf("\n=== %s ===\n", label)
	if len(props) == 0 {
		fmt.Println("  (none)")
		return
	}
	for k, v := range props {
		fmt.Printf("  %-35s %s\n", k+":", v)
	}
}

func main() {
	css, err := gpvskin.LoadCSS("https://gamepadviewer.com/style.css")
	if err != nil {
		panic(err)
	}

	p(".xbox .face.up (merged)", css.Lookup([][]string{{"controller", "xbox"}, {"face", "up"}}))
	p(".xbox .face.down (merged)", css.Lookup([][]string{{"controller", "xbox"}, {"face", "down"}}))
	p(".xbox .face.pressed", css.Lookup([][]string{{"controller", "xbox"}, {"face", "pressed"}}))
	p(".xbox .trigger.left (merged)", css.Lookup([][]string{{"controller", "xbox"}, {"trigger", "left"}}))
	p(".xbox .trigger.right (merged)", css.Lookup([][]string{{"controller", "xbox"}, {"trigger", "right"}}))
}
