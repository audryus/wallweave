build:
	go build -o bin/wallweave .

build-debug:
	go build -gcflags="all=-N -l" -o bin/wallweave .

run: build
	QML_IMPORT_PATH=/usr/share/omarchy/shell qs -p ui/_preview.qml

# App completa com símbolos no Go — anexe pelo VS Code
# (Run and Debug → "Attach to wallweave backend" → processo "wallweave")
debug: build-debug
	QML_IMPORT_PATH=/usr/share/omarchy/shell qs -p ui/_preview.qml
