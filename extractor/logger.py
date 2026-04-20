import logging
import structlog

def setup_logging(json_logs: bool = False):
    # Processors run on every log entry in order
    shared_processors = [
        structlog.contextvars.merge_contextvars,  # include request-scoped context
        structlog.stdlib.add_log_level,
        structlog.stdlib.add_logger_name,
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.StackInfoRenderer(),
        structlog.processors.format_exc_info,
    ]
    structlog.configure(
        processors=[
            *shared_processors,
            # Pass processed event dict to stdlib for rendering
            structlog.stdlib.ProcessorFormatter.wrap_for_formatter,
        ],
        # Output through stdlib logging
        logger_factory=structlog.stdlib.LoggerFactory(),
        wrapper_class=structlog.stdlib.BoundLogger,
        cache_logger_on_first_use=True,
    )

    # JSON for production, colored console output for development
    renderer = (
        structlog.processors.JSONRenderer()
        if json_logs else structlog.dev.ConsoleRenderer()
    )
    formatter = structlog.stdlib.ProcessorFormatter(
        processors=[
            structlog.stdlib.ProcessorFormatter.remove_processors_meta,
            renderer,
        ],
        # Run shared processors on stdlib logs
        foreign_pre_chain=shared_processors,
    )

    # Attach the formatter to stdlib's root logger
    handler = logging.StreamHandler()
    handler.setFormatter(formatter)
    logging.basicConfig(level=logging.INFO, handlers=[handler], force=True)

    # Let uvicorn's logs propagate so they go through structlog too
    for name in ("uvicorn", "uvicorn.error"):
        logging.getLogger(name).handlers = []
        logging.getLogger(name).propagate = True
