FROM python:3.12

RUN pip install \
        mkdocs-material \
        mkdocs-awesome-pages-plugin \
        click==8.2.1 # Workaround for live reload (cf. https://github.com/squidfunk/mkdocs-material/issues/8478)

USER 1000

WORKDIR /app

ENTRYPOINT ["mkdocs", "serve", "-w", "/app/docs", "--dev-addr", "0.0.0.0:8000"]
