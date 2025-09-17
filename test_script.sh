#!/bin/bash

# test_script.sh - Test script for S3 retention flag behavioral tests
# This script validates the S3 retention functionality added in PR #12669

set -e

# Default output path
OUTPUT_PATH="test_results.xml"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --output_path)
      OUTPUT_PATH="$2"
      shift 2
      ;;
    -h|--help)
      echo "Usage: $0 [--output_path <path>]"
      echo "  --output_path: Path to save JUnit XML output (default: test_results.xml)"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

echo "=== S3 Retention Flag Behavioral Tests ==="
echo "Testing PR #12669: Add retention flag specific for S3"
echo "Output will be saved to: $OUTPUT_PATH"
echo ""

# Ensure we're in the project root
if [[ ! -f "go.mod" ]]; then
  echo "Error: Must run from k3s project root directory"
  exit 1
fi

# Check if gotestsum is available for JUnit XML output
if ! command -v gotestsum &> /dev/null; then
  echo "Installing gotestsum for JUnit XML output..."
  go install gotest.tools/gotestsum@latest
  
  # Add GOPATH/bin to PATH for this session
  GOPATH_BIN=$(go env GOPATH)/bin
  export PATH="$PATH:$GOPATH_BIN"
fi

# Set up Go environment
export GO111MODULE=on
export CGO_ENABLED=0

echo "Running S3 retention behavioral tests..."

# Create temporary directory for test artifacts
TEMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'k3s-test')
trap "rm -rf $TEMP_DIR" EXIT

# Test packages to run
TEST_PACKAGES=(
  "./pkg/etcd/s3/..."
)

# Run tests with coverage and JUnit XML output
echo "Executing test suite..."

        # Run the optimized S3 retention tests (focused on critical behaviors)
        gotestsum --junitfile="$OUTPUT_PATH" --format=testname -- \
          -v \
          -timeout=2m \
          -coverprofile="$TEMP_DIR/coverage.out" \
          -run="TestS3Retention" \
          "${TEST_PACKAGES[@]}"

TEST_EXIT_CODE=$?

# Generate coverage report
if [[ -f "$TEMP_DIR/coverage.out" ]]; then
  echo ""
  echo "=== Test Coverage Report ==="
  go tool cover -func="$TEMP_DIR/coverage.out" | grep -E "(s3\.go|config_secret\.go)" || true
  
  # Generate HTML coverage report
  COVERAGE_HTML="$TEMP_DIR/coverage.html"
  go tool cover -html="$TEMP_DIR/coverage.out" -o="$COVERAGE_HTML"
  echo "Coverage HTML report generated: $COVERAGE_HTML"
fi

# Display results summary
echo ""
echo "=== Test Results Summary ==="
if [[ $TEST_EXIT_CODE -eq 0 ]]; then
  echo "✅ All S3 retention tests PASSED"
else
  echo "❌ Some S3 retention tests FAILED"
fi

echo "📊 JUnit XML report saved to: $OUTPUT_PATH"

# Validate JUnit XML was created
if [[ ! -f "$OUTPUT_PATH" ]]; then
  echo "⚠️  Warning: JUnit XML file was not created"
  exit 1
fi

# Show brief test statistics from JUnit XML
if command -v xmllint &> /dev/null; then
  echo ""
  echo "=== Test Statistics ==="
  TOTAL_TESTS=$(xmllint --xpath "count(//testcase)" "$OUTPUT_PATH" 2>/dev/null || echo "unknown")
  FAILED_TESTS=$(xmllint --xpath "count(//testcase/failure)" "$OUTPUT_PATH" 2>/dev/null || echo "0")
  SKIPPED_TESTS=$(xmllint --xpath "count(//testcase/skipped)" "$OUTPUT_PATH" 2>/dev/null || echo "0")
  
  echo "Total tests: $TOTAL_TESTS"
  echo "Failed tests: $FAILED_TESTS"
  echo "Skipped tests: $SKIPPED_TESTS"
  echo "Passed tests: $((TOTAL_TESTS - FAILED_TESTS - SKIPPED_TESTS))"
fi

echo ""
echo "=== Optimized Behavioral Test Validation ==="
echo "These tests validate the most critical behaviors:"
echo "1. ✓ S3 retention operates independently (core PR behavior)"
echo "2. ✓ S3 retention is applied correctly based on configured values"
echo "3. ✓ S3 retention defaults to 5 when not specified"
echo "4. ✓ S3 retention can be disabled (set to 0)"
echo "5. ✓ S3 retention can be configured via Kubernetes secrets"
echo "6. ✓ S3 retention handles invalid secret values gracefully"

exit $TEST_EXIT_CODE
