package main

import (
	"os"
	"text/template"
)

// Custom functions for template
var funcMap = template.FuncMap{
	"loop":        loop,
	"add":         func(a, b int) int { return a + b },
	"sub":         func(a, b int) int { return a - b },
	"mul":         func(a, b int) int { return a * b },
	"div":         func(a, b int) int { return a / b },
	"mod":         func(a, b int) int { return a % b },
	"isSuperNode": func(nodeID int) bool { return nodeID%10 == 5 },
}

func loop(start, end int) []int {
	var items []int
	for i := start; i <= end; i++ {
		items = append(items, i)
	}
	return items
}

type Config struct {
	TotalNodes int
	NumGroups  int
}

func main() {
	config := Config{
		TotalNodes: 100, // Всего нод
		NumGroups:  3,   // Количество групп
	}

	// Генерация docker-compose.yml
	generateFile("docker-compose.tmpl", "docker-compose.yml", config)

	// Генерация prometheus.yml
	generateFile("prometheus.yml.tmpl", "prometheus.yml", config)
}

func generateFile(templateFile, outputFile string, config Config) {
	tmpl := template.Must(template.New(templateFile).Funcs(funcMap).ParseFiles(templateFile))

	f, err := os.Create(outputFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	err = tmpl.Execute(f, config)
	if err != nil {
		panic(err)
	}
}
