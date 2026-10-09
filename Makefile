test:
	python3 -m unittest discover -s tests -t .

# Preview with English (default).
run:
	QML_IMPORT_PATH=/usr/share/omarchy/shell qs -p ui/_preview.qml

# Preview forcing Portuguese (works without generating pt_BR locale).
run-pt:
	QML_IMPORT_PATH=/usr/share/omarchy/shell WALLWEAVE_LANG=pt qs -p ui/_preview.qml

# Preview forcing Chinese.
run-zh:
	QML_IMPORT_PATH=/usr/share/omarchy/shell WALLWEAVE_LANG=zh qs -p ui/_preview.qml

validate:
	omarchy plugin validate .

rescan:
	omarchy-shell shell rescanPlugins

restart: 
	omarchy restart shell
