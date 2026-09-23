build:
	go build -o bin/wallweave .

build-debug:
	go build -gcflags="all=-N -l" -o bin/wallweave .

# Preview with English (default).
run: build
	QML_IMPORT_PATH=/usr/share/omarchy/shell qs -p ui/_preview.qml

# Preview forcing Portuguese (works without generating pt_BR locale).
run-pt: build
	QML_IMPORT_PATH=/usr/share/omarchy/shell WALLWEAVE_LANG=pt qs -p ui/_preview.qml

# Preview forcing Chinese.
run-zh: build
	QML_IMPORT_PATH=/usr/share/omarchy/shell WALLWEAVE_LANG=zh qs -p ui/_preview.qml

debug: build-debug
	QML_IMPORT_PATH=/usr/share/omarchy/shell qs -p ui/_preview.qml

validate:
	omarchy plugin validate .

rescan:
	omarchy-shell shell rescanPlugins

restart: 
	omarchy restart shell
