if (-not (Test-Path .\go.sum)) {
  go mod tidy
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

go run .
