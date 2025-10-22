.PHONY: fmt gen clean

# Format code
fmt:
	goimports -w .

# Generate protobuf code
gen:
	buf generate

# Clean generated code
clean:
	rm -rf gen/
