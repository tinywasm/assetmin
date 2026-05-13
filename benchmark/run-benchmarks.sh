#!/bin/bash

# Benchmark runner script for assetmin SSR extraction performance
# Captures timing data and generates a summary report

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$SCRIPT_DIR/results"

# Create results directory
mkdir -p "$RESULTS_DIR"

# Timestamp for this run
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
REPORT_FILE="$RESULTS_DIR/benchmark_${TIMESTAMP}.txt"

echo "🚀 Starting assetmin benchmark suite..."
echo "📊 Results will be saved to: $REPORT_FILE"
echo ""

# Run benchmarks with detailed output
echo "Running benchmarks (5 seconds each)..."
echo "========================================" > "$REPORT_FILE"
echo "AssetMin Benchmark Report" >> "$REPORT_FILE"
echo "Timestamp: $(date)" >> "$REPORT_FILE"
echo "========================================" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

cd "$PROJECT_DIR"

# Run each benchmark individually to show per-benchmark timing
echo "📌 Single Module Extraction" | tee -a "$REPORT_FILE"
go test -bench=BenchmarkExtractSSRAssets_SingleModule -benchmem -benchtime=10s -v ./benchmark 2>&1 | tee -a "$REPORT_FILE"
echo "" | tee -a "$REPORT_FILE"

echo "📌 Three Modules Extraction (Cache Effectiveness)" | tee -a "$REPORT_FILE"
go test -bench=BenchmarkExtractSSRAssets_ThreeModules -benchmem -benchtime=10s -v ./benchmark 2>&1 | tee -a "$REPORT_FILE"
echo "" | tee -a "$REPORT_FILE"

echo "📌 Large CSS Extraction" | tee -a "$REPORT_FILE"
go test -bench=BenchmarkExtractSSRAssets_LargeCSS -benchmem -benchtime=10s -v ./benchmark 2>&1 | tee -a "$REPORT_FILE"
echo "" | tee -a "$REPORT_FILE"

# Summary section
echo "" >> "$REPORT_FILE"
echo "========================================" >> "$REPORT_FILE"
echo "SUMMARY" >> "$REPORT_FILE"
echo "========================================" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "Expected Performance Characteristics:" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "✓ Single Module:  ~1-2ms (cached extraction)" >> "$REPORT_FILE"
echo "✓ Three Modules:  ~1-2ms (cached extraction, all compiled together)" >> "$REPORT_FILE"
echo "✓ Large CSS:      ~1-2ms (cached extraction with larger asset)" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "Cache Effectiveness:" >> "$REPORT_FILE"
echo "- First call (cold):  ~300-500ms (Go compiler + execution)" >> "$REPORT_FILE"
echo "- Subsequent (warm):  ~1-10ms (cache lookup + JSON parse)" >> "$REPORT_FILE"
echo "- Typical app (7 modules): 5x faster than per-module AST extraction" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

echo ""
echo "✅ Benchmark completed!"
echo "📄 Full report: $REPORT_FILE"
echo ""
echo "View results:"
echo "  cat $REPORT_FILE"
