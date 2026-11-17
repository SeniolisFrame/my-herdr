package ui

// thaiKeys maps what the Thai Kedmanee layout types to the Latin key in the
// same position, so the explorer's shortcuts work without switching layout.
// Used by handleKey (model.go) and pickKey (ops.go) before their key switch.
var thaiKeys = map[string]string{
	// Explorer shortcuts (unshifted keys, plus Shift+G for G).
	"ๆ": "q", "ห": "s", "แ": "c", "ฟ": "a",
	"่": "j", "า": "k", "้": "h", "ส": "l",
	"เ": "g", "ฌ": "G", "ื": "n", "ย": "p", "น": "o",
	"บ": "[", "ล": "]",
	// Fork-like actions (shifted keys use the Kedmanee shift layer).
	"ฆ": "S", "๊": "U", "ฏ": "D", "ิ": "b", "ฺ": "B", "ท": "m",
	"ฯ": "O", "ธ": "T", "ด": "f", "โ": "F", "ญ": "P",
	"ผ": "z", "ศ": "L", "ฦ": "?",
	// Kedmanee Shift+X / Shift+Z type plain parentheses, so on the English
	// layout "(" and ")" also trigger Z and X.
	")": "X", "(": "Z",
	// Search: / and Shift+N.
	"ฝ": "/", "์": "N",
}
