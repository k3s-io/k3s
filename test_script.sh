#!/bin/bash

set -e

# S3 Retention Test Suite for PR #12669
# This script tests the S3 retention functionality introduced in PR #12669
# 
# The tests are written from the base commit perspective and should:
# - FAIL on base commit (02c898dcc1) - before PR changes
# - PASS on merge commit (a238f33cdd) - with PR changes

# Default output path
OUTPUT_PATH="test-results.xml"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --output_path)
      OUTPUT_PATH="$2"
      shift 2
      ;;
    -h|--help)
      echo "Usage: $0 [--output_path <path>]"
      echo ""
      echo "Options:"
      echo "  --output_path <path>  Path to save JUnit XML test results (default: test-results.xml)"
      echo "  -h, --help           Show this help message"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      echo "Use -h or --help for usage information"
      exit 1
      ;;
  esac
done

# Ensure output directory exists
OUTPUT_DIR=$(dirname "$OUTPUT_PATH")
mkdir -p "$OUTPUT_DIR"

echo "🧪 S3 Retention Test Suite for PR #12669"
echo "=========================================="
echo "Output path: $OUTPUT_PATH"
echo ""

# Check if we're in the k3s repository
if [[ ! -f "go.mod" ]] || ! grep -q "github.com/k3s-io/k3s" go.mod; then
    echo "❌ Error: This script must be run from the root of the k3s repository"
    exit 1
fi

# Check for Go installation
if ! command -v go &> /dev/null; then
    echo "❌ Error: Go is not installed or not in PATH"
    exit 1
fi

echo "✅ Go version: $(go version)"

# Install gotestsum if not available
if ! command -v gotestsum &> /dev/null; then
    echo "📦 Installing gotestsum..."
    go install gotest.tools/gotestsum@latest
    
    # Check if it's still not in PATH (might be in GOPATH/bin)
    if ! command -v gotestsum &> /dev/null; then
        # Try to find it in common Go paths
        GOPATH_BIN="${GOPATH:-$HOME/go}/bin"
        if [[ -x "$GOPATH_BIN/gotestsum" ]]; then
            export PATH="$GOPATH_BIN:$PATH"
            echo "✅ Added $GOPATH_BIN to PATH"
        else
            echo "❌ Error: Could not install or find gotestsum"
            exit 1
        fi
    fi
fi

echo "✅ gotestsum version: $(gotestsum --version)"
echo ""

# Create temporary directory for coverage
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Test packages to run - focusing on our specific retention tests
TEST_PACKAGES=(
  "./pkg/etcd/s3"
)

echo "🔍 Test Summary:"
echo "- Testing S3 retention functionality from PR #12669"
echo "- Base commit (02c898dcc1): Tests should FAIL (no Retention field)"
echo "- Merge commit (a238f33cdd): Tests should PASS (has Retention field)"
echo "- Focus: EtcdS3 struct changes and SnapshotRetention signature changes"
echo ""

# Run tests with coverage and JUnit XML output
echo "🚀 Executing S3 retention test suite..."
echo ""

# Run only our specific retention tests
gotestsum --junitfile="$OUTPUT_PATH" --format=testname -- \
  -v \
  -timeout=2m \
  -coverprofile="$TEMP_DIR/coverage.out" \
  -run="TestEtcdS3Retention|TestSnapshotRetentionSignature|TestS3Retention|TestDefaultEtcdS3Retention" \
  "${TEST_PACKAGES[@]}"

TEST_EXIT_CODE=$?

echo ""
echo "📊 Test Results Summary:"
echo "========================"

if [[ $TEST_EXIT_CODE -eq 0 ]]; then
    echo "✅ All tests passed!"
    echo ""
    echo "🎯 Expected Behavior:"
    echo "- If running on MERGE COMMIT (a238f33cdd): ✅ PASS (expected)"
    echo "- If running on BASE COMMIT (02c898dcc1): ❌ This should have FAILED!"
    echo ""
    echo "📋 Behavioral Test Validation:"
    echo "- EtcdS3 struct has Retention field"
    echo "- SnapshotRetention function signature uses config-based retention"
    echo "- S3 config secrets handle etcd-s3-retention key"
    echo "- Default S3 config includes retention settings"
else
    echo "❌ Some tests failed (exit code: $TEST_EXIT_CODE)"
    echo ""
    echo "🎯 Expected Behavior:"
    echo "- If running on BASE COMMIT (02c898dcc1): ❌ FAIL (expected)"
    echo "- If running on MERGE COMMIT (a238f33cdd): ✅ This should have PASSED!"
    echo ""
    echo "📋 Behavioral Test Validation:"
    echo "- Tests are checking for S3 retention functionality"
    echo "- Failures indicate missing Retention field or old function signatures"
    echo "- This confirms the PR introduced the expected changes"
fi

# Show coverage information if available
if [[ -f "$TEMP_DIR/coverage.out" ]]; then
    echo ""
    echo "📈 Coverage Report:"
    go tool cover -func="$TEMP_DIR/coverage.out" | grep -E "(total|s3\.go)" || true
fi

echo ""
echo "📄 JUnit XML results saved to: $OUTPUT_PATH"
echo ""

# Current git commit info
echo "🔍 Current Git State:"
CURRENT_COMMIT=$(git rev-parse HEAD)
CURRENT_SHORT=$(git rev-parse --short HEAD)
COMMIT_MESSAGE=$(git log -1 --pretty=format:"%s")

echo "- Commit: $CURRENT_COMMIT"
echo "- Short: $CURRENT_SHORT" 
echo "- Message: $COMMIT_MESSAGE"

# Check if this is the expected base or merge commit
if [[ "$CURRENT_COMMIT" == "02c898dcc1"* ]]; then
    echo "- Status: 🔵 BASE COMMIT (before PR #12669)"
    echo "- Expected: Tests should FAIL"
elif [[ "$CURRENT_COMMIT" == "a238f33cdd"* ]]; then
    echo "- Status: 🟢 MERGE COMMIT (with PR #12669)"
    echo "- Expected: Tests should PASS"
else
    echo "- Status: ⚪ OTHER COMMIT"
    echo "- Note: Not the specific base or merge commit for PR #12669"
fi

echo ""
echo "✨ Test suite execution completed!"

exit $TEST_EXIT_CODE