# Запуск бенчмарков и сохранение результатов
bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem -count=10 -benchtime=10x ./simulator > benchmark_results.txt

# Анализ результатов с помощью benchstat
benchstat: benchmark_results.txt
	@echo "Analyzing benchmark results..."
	@go install golang.org/x/perf/cmd/benchstat@latest
	@benchstat benchmark_results.txt > benchstat.txt

# Очистка результатов
clean:
	@rm -f benchmark_results.txt

# Запуск всего процесса
all: clean bench benchstat 