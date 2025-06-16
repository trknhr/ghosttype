#!/bin/bash

sudo purge
echo "After cache clear:"
time ./bin/ghosttype-optimized --quick-exit

echo "Second run (cached):"
time ./bin/ghosttype-optimized --quick-exit


# measure_ghosttype.sh
echo "=== Ghosttype startup time measurement ==="
for i in {1..10}; do
    echo -n "Run $i: "
    sleep 1
    /usr/bin/time -p ./bin/ghosttype-optimized --quick-exit 2>&1 | grep real
done