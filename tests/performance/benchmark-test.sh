#!/bin/bash
# Go benchmark runner for HSM Service
# Runs all benchmarks and generates reports

set -euo pipefail

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}HSM Service - Go Benchmark Suite${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Configuration
BENCH_TIME="${BENCH_TIME:-3s}"
BENCH_COUNT="${BENCH_COUNT:-5}"
RESULTS_DIR="./benchmark-results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

mkdir -p "$RESULTS_DIR"

echo -e "${BLUE}[INFO]${NC} Running benchmarks..."
echo -e "${BLUE}[INFO]${NC} Duration per benchmark: $BENCH_TIME"
echo -e "${BLUE}[INFO]${NC} Iterations: $BENCH_COUNT"
echo -e "${BLUE}[INFO]${NC} Results: $RESULTS_DIR/bench_$TIMESTAMP.txt"
echo ""

# Run benchmarks
cd /home/leon/docker/ct-system/hsm-service

go test ./internal/hsm/... \
    -bench=. \
    -benchtime="$BENCH_TIME" \
    -count="$BENCH_COUNT" \
    -benchmem \
    -run=^$ \
    2>&1 | tee "$RESULTS_DIR/bench_$TIMESTAMP.txt"

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Benchmark Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# Summary
echo -e "${BLUE}Summary:${NC}"
echo ""
grep "^Benchmark" "$RESULTS_DIR/bench_$TIMESTAMP.txt" | awk '{
    name=$1
    gsub(/^Benchmark/, "", name)
    gsub(/-[0-9]+$/, "", name)
    
    ns=$3
    mem=$5
    allocs=$7
    
    # Convert ns/op to readable format
    if (ns < 1000) {
        time=sprintf("%.2f ns/op", ns)
    } else if (ns < 1000000) {
        time=sprintf("%.2f μs/op", ns/1000)
    } else {
        time=sprintf("%.2f ms/op", ns/1000000)
    }
    
    # Convert memory to readable format
    if (mem < 1024) {
        memory=sprintf("%s B/op", mem)
    } else {
        memory=sprintf("%.2f KB/op", mem/1024)
    }
    
    printf "  %-30s %12s  %10s  %s\n", name":", time, memory, allocs" allocs/op"
}'

echo ""
echo -e "${YELLOW}Analysis:${NC}"
echo ""

# Check for performance regressions (if previous results exist)
PREV_RESULT=$(ls -t "$RESULTS_DIR"/bench_*.txt 2>/dev/null | sed -n 2p)
if [ -n "$PREV_RESULT" ]; then
    echo -e "${BLUE}[INFO]${NC} Comparing with previous run: $(basename $PREV_RESULT)"
    echo ""
    
    # Extract average times from current and previous runs
    CURRENT_AVG=$(grep "^Benchmark" "$RESULTS_DIR/bench_$TIMESTAMP.txt" | awk '{sum+=$3; count++} END {print sum/count}')
    PREV_AVG=$(grep "^Benchmark" "$PREV_RESULT" | awk '{sum+=$3; count++} END {print sum/count}')
    
    if (( $(echo "$CURRENT_AVG > $PREV_AVG * 1.1" | bc -l) )); then
        echo -e "${YELLOW}[!]${NC} Performance regression detected (>10% slower)"
        echo "    Previous avg: $(printf "%.2f" $PREV_AVG) ns/op"
        echo "    Current avg:  $(printf "%.2f" $CURRENT_AVG) ns/op"
    elif (( $(echo "$CURRENT_AVG < $PREV_AVG * 0.9" | bc -l) )); then
        echo -e "${GREEN}[✓]${NC} Performance improvement detected (>10% faster)"
        echo "    Previous avg: $(printf "%.2f" $PREV_AVG) ns/op"
        echo "    Current avg:  $(printf "%.2f" $CURRENT_AVG) ns/op"
    else
        echo -e "${GREEN}[✓]${NC} Performance is stable (within 10% of previous run)"
    fi
    echo ""
else
    echo -e "${BLUE}[INFO]${NC} No previous results found for comparison"
    echo ""
fi

# Generate benchstat comparison (if available)
if command -v benchstat &> /dev/null && [ -n "$PREV_RESULT" ]; then
    echo -e "${BLUE}[INFO]${NC} Running benchstat comparison..."
    echo ""
    benchstat "$PREV_RESULT" "$RESULTS_DIR/bench_$TIMESTAMP.txt"
    echo ""
fi

echo -e "${BLUE}Tips:${NC}"
echo "  - Install benchstat for detailed comparison: go install golang.org/x/perf/cmd/benchstat@latest"
echo "  - Run CPU profiling: go test -bench=. -cpuprofile=cpu.prof"
echo "  - Run memory profiling: go test -bench=. -memprofile=mem.prof"
echo "  - Analyze profile: go tool pprof cpu.prof"
echo ""
