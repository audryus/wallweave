OMARCHY_SHELL ?= /usr/share/omarchy/shell

test:
	python3 -m unittest discover -s tests -t .

# Dev-only symlinks so qs/the editor resolve "import qs.Commons" / "qs.Ui"
# from ui/. Not committed (the plugin validator rejects symlinks); inside
# Omarchy the shell provides these imports itself.
links:
	ln -sfn $(OMARCHY_SHELL)/Commons ui/Commons
	ln -sfn $(OMARCHY_SHELL)/Ui ui/Ui
	mkdir -p ui/qs
	ln -sfn $(OMARCHY_SHELL) ui/qs/shell

unlinks:
	rm -f ui/Commons ui/Ui ui/qs/shell
	rmdir ui/qs 2>/dev/null || true

# Preview with English (default).
run: links
	QML_IMPORT_PATH=/usr/share/omarchy/shell qs -p ui/_preview.qml

# Preview forcing Portuguese (works without generating pt_BR locale).
run-pt: links
	QML_IMPORT_PATH=/usr/share/omarchy/shell WALLWEAVE_LANG=pt qs -p ui/_preview.qml

# Preview forcing Chinese.
run-zh: links
	QML_IMPORT_PATH=/usr/share/omarchy/shell WALLWEAVE_LANG=zh qs -p ui/_preview.qml

validate: unlinks
	omarchy plugin validate .

rescan:
	omarchy-shell shell rescanPlugins

restart: 
	omarchy restart shell
