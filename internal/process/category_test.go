package process

import "testing"

func TestDefaultClassifier(t *testing.T) {
	classifier := DefaultClassifier()
	tests := []struct {
		command string
		want    Category
	}{
		{"systemd", CategorySystem},
		{"Docker", CategoryContainer},
		{"chrome.exe", CategoryBrowser},
		{"redis-server", CategoryDatabase},
		{"NODE", CategoryDevelopment},
		{"java", CategoryJVM},
		{"editor", CategoryOther},
	}
	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			if got := classifier.Classify(Info{Command: test.command}); got != test.want {
				t.Fatalf("Classify(%q) = %q; want %q", test.command, got, test.want)
			}
		})
	}
}
