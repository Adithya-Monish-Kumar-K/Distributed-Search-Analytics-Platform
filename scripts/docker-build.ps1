#!/usr/bin/env pwsh
# Build all Docker images for the search platform

$ErrorActionPreference = "Stop"

$services = @("ingestion", "indexer", "searcher")
$tag = if ($args[0]) { $args[0] } else { "latest" }

Write-Host "=== Building Search Platform Docker Images ===" -ForegroundColor Cyan
Write-Host "Tag: $tag" -ForegroundColor Yellow
Write-Host ""

foreach ($svc in $services) {
    Write-Host "Building searchplatform-${svc}:${tag}..." -ForegroundColor Green
    docker build `
        -f "deployments/docker/Dockerfile.$svc" `
        -t "searchplatform-${svc}:${tag}" `
        .
    
    if ($LASTEXITCODE -ne 0) {
        Write-Host "FAILED to build $svc" -ForegroundColor Red
        exit 1
    }
    Write-Host "  Done." -ForegroundColor Green
}

Write-Host ""
Write-Host "=== All images built ===" -ForegroundColor Cyan
docker images "searchplatform-*" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"