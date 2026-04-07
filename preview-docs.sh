#!/bin/bash
# Simple documentation preview script

echo "Starting documentation preview server..."
echo "Open http://localhost:8000/docs/ in your browser"
echo "Press Ctrl+C to stop"
echo ""

cd "$(dirname "$0")" || exit
python3 -m http.server 8000
