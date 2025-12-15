#!/bin/sh
set -e

# Start Ollama server in background
/bin/ollama serve &

# Wait for Ollama to be ready
echo "Waiting for Ollama to start..."
sleep 15

# Check if Ollama is ready
for i in 1 2 3 4 5; do
  if curl -f http://localhost:11434/api/tags >/dev/null 2>&1; then
    echo "Ollama is ready"
    break
  fi
  echo "Waiting for Ollama... attempt $i"
  sleep 5
done

# Pull model if not exists
echo "Checking for deepseek-coder model..."
if ! ollama list | grep -q "deepseek-coder"; then
  echo "Pulling deepseek-coder model..."
  ollama pull deepseek-coder || echo "Failed to pull model, but continuing..."
else
  echo "Model deepseek-coder already exists"
fi

# Wait for the server process
wait
