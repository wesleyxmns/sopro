package tui

type keyHint struct {
	key   string
	label string
}

var defaultHints = []keyHint{
	{"↑↓", "navegar"},
	{"p", "pausar/retomar"},
	{"x", "encerrar"},
	{"k", "forçar"},
	{"c", "limpar cache"},
	{"/", "buscar"},
	{"f", "filtrar"},
	{"s", "ordenar"},
	{"g", "agrupar"},
	{"q", "sair"},
}

func hintsForWidth(width int, pending bool) []keyHint {
	if pending {
		return []keyHint{{"enter/y", "confirmar"}, {"esc/n", "cancelar"}}
	}
	if width < 60 {
		return []keyHint{{"↑↓", "nav"}, {"x", "term"}, {"k", "forçar"}, {"q", "sair"}}
	}
	hints := make([]keyHint, 0, len(defaultHints))
	for _, hint := range defaultHints {
		if width < 110 && (hint.key == "p" || hint.key == "c" || hint.key == "/" || hint.key == "f" || hint.key == "s" || hint.key == "g") {
			continue
		}
		if width < 145 && (hint.key == "s" || hint.key == "g") {
			continue
		}
		hints = append(hints, hint)
	}
	return hints
}
