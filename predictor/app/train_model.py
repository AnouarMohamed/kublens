"""Production-oriented CLI entrypoint for predictor model training."""

from __future__ import annotations

from predictor.app.ml_prototype import main as train_main


def main() -> None:
    """Run predictor model training."""

    train_main()


if __name__ == "__main__":
    main()
