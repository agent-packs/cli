---
name: ml-experiment-tracking
description: Make ML experiments reproducible and comparable by tracking hyperparameters, metrics, artifacts, and environments. Use when training models, tuning hyperparameters, or promoting models to production.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.1.0"
---

# ML Experiment Tracking

An unreproducible experiment is not an experiment — it's a one-off. Track everything from the start.

## What to Track

Track these for every run, automatically:

- **Hyperparameters** — learning rate, batch size, architecture choices, regularization coefficients. Log before training starts, not after.
- **Metrics** — loss and task metrics (accuracy, F1, BLEU, etc.) at every epoch for train, validation, and test splits. Log step-level metrics for long runs.
- **Artifacts** — model checkpoints, tokenizers, preprocessors, evaluation outputs. Link artifacts to the run that produced them.
- **Environment** — Python version, library versions (`pip freeze`), CUDA version, hardware (GPU model, memory). Reproduce without this and you'll get different numbers.
- **Dataset version** — hash or DVC tag of the dataset used. A model is only as reproducible as its data.
- **Code version** — git commit SHA at the time of the run. If the repo is dirty, log the diff.

## Tooling

- **MLflow** — self-hosted, integrates with most frameworks, good for teams already on Databricks.
- **Weights & Biases (wandb)** — best UX for experiment comparison, hyperparameter sweep visualization, and collaborative analysis.
- **DVC** — version datasets and models with git-like semantics; integrates with S3/GCS/Azure for artifact storage.
- **Sacred** — lightweight, config-driven, good for academic reproducibility.

Use one tool per project. Mixed tracking produces orphaned experiments that can't be compared.

## Experiment Design

- Establish a baseline run first — a simple model, no tuning — and commit its results. All subsequent experiments are measured against this baseline.
- Change one thing at a time. Multi-variable experiments make it impossible to attribute the improvement (or regression).
- Name experiments with purpose: `lr-sweep-transformer-base`, not `experiment-47`. Include the hypothesis in the description field.
- Log the full config object, not just the parameters you think matter. You will change your mind about what matters.

## Reproducibility

- Set all random seeds explicitly: Python `random`, NumPy, PyTorch/TF, and CUDA: `torch.manual_seed(42)`, `torch.cuda.manual_seed_all(42)`, `torch.backends.cudnn.deterministic = True`.
- Use deterministic data loaders: `DataLoader(shuffle=False)` for evaluation, fixed-seed shuffling for training.
- Pin library versions in `requirements.txt` or `pyproject.toml`. Floating `torch>=2.0` is a reproducibility time bomb.
- Save the full model config (architecture + tokenizer config) alongside weights, not just the weight file. You cannot reload a model without its config.

## Model Registry and Promotion

- Use a model registry (MLflow Model Registry, W&B Artifacts, SageMaker Model Registry) to track the lifecycle: `staging` → `production` → `archived`.
- Promotion criteria should be explicit and automated: "new model must beat baseline by ≥1% F1 on holdout set with p<0.05."
- Link every production model version to the experiment run, dataset version, and git SHA that produced it. Audit trails are required for regulated domains.
- Test models with a model evaluation CI step before promotion — the same way code has a test suite.

## Checklist

- [ ] Hyperparameters, metrics, and environment logged automatically for every run.
- [ ] Dataset version (hash or DVC tag) recorded with each run.
- [ ] Git commit SHA logged; run blocked if repo is dirty (for production runs).
- [ ] All random seeds set and logged.
- [ ] Baseline run committed and tagged in the experiment tracker.
- [ ] Production model linked to its experiment run in the model registry.
