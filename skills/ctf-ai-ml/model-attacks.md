# CTF AI/ML - Model Attacks

Techniques for attacking ML models directly: weight manipulation, model inversion, encoder collision, LoRA adapter exploitation, model extraction, and membership inference. For adversarial example generation and data poisoning, see [adversarial-ml.md](adversarial-ml.md). For LLM-specific attacks, see [llm-attacks.md](llm-attacks.md).

## Table of Contents
- [ML Model Weight Perturbation Negation (DiceCTF 2026)](#ml-model-weight-perturbation-negation-dicectf-2026)
- [ML Model Inversion via Gradient Descent (BSidesSF 2025)](#ml-model-inversion-via-gradient-descent-bsidessf-2025)
- [Neural Network Encoder Collision (RootAccess2026)](#neural-network-encoder-collision-rootaccess2026)
- [LoRA Adapter Weight Merging (ApoorvCTF 2026)](#lora-adapter-weight-merging-apoorvctf-2026)
- [Model Extraction via Query API](#model-extraction-via-query-api)
- [Membership Inference Attack](#membership-inference-attack)

---

## ML Model Weight Perturbation Negation (DiceCTF 2026)

**Pattern:** A GPT-2 model has been fine-tuned to suppress a specific behavior (e.g., generating a flag). The challenge provides both the original base model and the fine-tuned (suppressed) model. By computing the weight delta and negating it, you reverse the suppression into amplification.

The key mathematical insight: if `W_chal = W_orig + delta` where `delta` was learned to suppress output, then `W_recovered = W_orig - delta = 2*W_orig - W_chal` amplifies the original behavior instead.

```python
import torch
from transformers import GPT2LMHeadModel, GPT2Tokenizer

# Load both models
original = GPT2LMHeadModel.from_pretrained("gpt2")
challenge = GPT2LMHeadModel.from_pretrained("./challenge_model")

# Compute negated weights: W_recovered = 2*W_orig - W_chal
recovered = GPT2LMHeadModel.from_pretrained("gpt2")
orig_sd = original.state_dict()
chal_sd = challenge.state_dict()
rec_sd = recovered.state_dict()

for key in orig_sd:
    rec_sd[key] = 2 * orig_sd[key] - chal_sd[key]

recovered.load_state_dict(rec_sd)

# Generate with recovered model
tokenizer = GPT2Tokenizer.from_pretrained("gpt2")
tokenizer.pad_token = tokenizer.eos_token

prompt = "The flag is"
inputs = tokenizer(prompt, return_tensors="pt")
with torch.no_grad():
    output = recovered.generate(
        **inputs,
        max_new_tokens=100,
        temperature=0.7,
        do_sample=True,
        num_return_sequences=5,
    )

for seq in output:
    print(tokenizer.decode(seq, skip_special_tokens=True))
```

**Key insight:** Fine-tuning adds a delta to weights. Negating that delta reverses suppression into amplification. Check which layers differ most with `(orig_sd[k] - chal_sd[k]).abs().max()` to confirm fine-tuning targeted specific layers. If only certain layers changed, the delta is sparse and negation is even more effective.

**Variations:**
- **Partial negation with scaling:** Sometimes `W_orig + alpha * (W_orig - W_chal)` with `alpha > 1` works better than pure negation. Try `alpha` values from 1.0 to 3.0.
- **Layer-selective negation:** Only negate layers that show significant delta (threshold by L2 norm of difference).
- **LoRA-aware negation:** If fine-tuning used LoRA, the delta is low-rank. Extract and negate only the LoRA component.

---

## ML Model Inversion via Gradient Descent (BSidesSF 2025)

**Pattern:** Given a trained model and a target output (e.g., a specific class label or embedding vector), recover the input by optimizing a random input tensor using gradient descent to minimize the distance between the model's output and the target.

```python
import torch
import torch.nn as nn
import torch.optim as optim
from torchvision import transforms
from PIL import Image

# Load the challenge model
model = torch.load("challenge_model.pt", map_location="cpu")
model.eval()

# Target: the output we want to invert (e.g., a specific embedding or class)
target_output = torch.load("target_embedding.pt")  # shape depends on model

# Initialize random input (e.g., 3x224x224 image)
input_tensor = torch.randn(1, 3, 224, 224, requires_grad=True)

optimizer = optim.Adam([input_tensor], lr=0.01)
mse_loss = nn.MSELoss()

for step in range(2000):
    optimizer.zero_grad()
    output = model(input_tensor)
    loss = mse_loss(output, target_output)

    # Optional: add total variation regularization for smoother images
    tv_loss = (
        torch.sum(torch.abs(input_tensor[:, :, :, :-1] - input_tensor[:, :, :, 1:])) +
        torch.sum(torch.abs(input_tensor[:, :, :-1, :] - input_tensor[:, :, 1:, :]))
    )
    total_loss = loss + 1e-4 * tv_loss

    total_loss.backward()
    optimizer.step()

    # Clamp to valid image range
    with torch.no_grad():
        input_tensor.clamp_(0, 1)

    if step % 200 == 0:
        print(f"Step {step}: loss={loss.item():.6f}")

# Save recovered image
recovered = input_tensor.squeeze(0).detach()
img = transforms.ToPILImage()(recovered)
img.save("recovered_input.png")
print("Recovered input saved to recovered_input.png")
```

**Key insight:** Neural networks are differentiable, so you can backpropagate through them to optimize the input. Total variation regularization produces more natural-looking images. If the model has batch normalization, set it to eval mode (`model.eval()`) to use running statistics rather than batch statistics.

**Variations:**
- **Feature visualization:** Maximize a specific neuron's activation instead of matching a target output.
- **Deep Dream style:** Use layer activations as optimization targets for artistic reconstruction.
- **Gradient-free inversion:** If gradients are unavailable (black-box), use CMA-ES or other evolutionary strategies.

---

## Neural Network Encoder Collision (RootAccess2026)

**Pattern:** Given a neural network encoder, find two distinct inputs that produce identical (or nearly identical) output embeddings. This exploits the dimensionality reduction inherent in encoders — the mapping from high-dimensional input to lower-dimensional embedding space is not injective.

```python
import torch
import torch.nn as nn
import torch.optim as optim

# Load the encoder model
encoder = torch.load("encoder.pt", map_location="cpu")
encoder.eval()

# Initialize two random inputs
input_a = torch.randn(1, 3, 64, 64, requires_grad=True)
input_b = torch.randn(1, 3, 64, 64, requires_grad=True)

optimizer = optim.Adam([input_a, input_b], lr=0.005)

for step in range(5000):
    optimizer.zero_grad()

    emb_a = encoder(input_a)
    emb_b = encoder(input_b)

    # Minimize distance between embeddings
    collision_loss = nn.MSELoss()(emb_a, emb_b)

    # Maximize distance between inputs (so they are distinct)
    input_diff = nn.MSELoss()(input_a, input_b)
    diversity_loss = -input_diff  # negative because we want to maximize

    # Regularize to valid range
    range_penalty = (
        torch.relu(-input_a).sum() + torch.relu(input_a - 1).sum() +
        torch.relu(-input_b).sum() + torch.relu(input_b - 1).sum()
    )

    loss = collision_loss + 0.1 * diversity_loss + 0.01 * range_penalty
    loss.backward()
    optimizer.step()

    with torch.no_grad():
        input_a.clamp_(0, 1)
        input_b.clamp_(0, 1)

    if step % 500 == 0:
        dist = (emb_a - emb_b).norm().item()
        inp_dist = (input_a - input_b).norm().item()
        print(f"Step {step}: emb_dist={dist:.8f}, input_dist={inp_dist:.4f}")

# Verify collision
with torch.no_grad():
    final_a = encoder(input_a)
    final_b = encoder(input_b)
    print(f"Final embedding distance: {(final_a - final_b).norm().item():.10f}")
    print(f"Final input distance: {(input_a - input_b).norm().item():.4f}")
    print(f"Embeddings equal: {torch.allclose(final_a, final_b, atol=1e-6)}")
```

**Key insight:** Encoders compress information, so collisions must exist by the pigeonhole principle. The optimization simultaneously minimizes embedding distance while maximizing input distance. Starting with very different random initializations helps avoid trivial solutions.

**Variations:**
- **Targeted collision:** Force both inputs to map to a specific target embedding.
- **Near-collision with hamming constraint:** Find inputs that differ in only a few pixels but produce identical embeddings.
- **Hash-like collision:** If the encoder output is discretized (e.g., binary hash), collision search is easier via relaxation.

---

## LoRA Adapter Weight Merging (ApoorvCTF 2026)

**Pattern:** A LoRA (Low-Rank Adaptation) adapter is provided alongside a base model. The adapter encodes hidden information in its low-rank weight matrices. Merging the adapter into the base model and generating output (or visualizing weight patterns) reveals the flag.

LoRA modifies weights as: `W_merged = W_base + alpha * (B @ A)` where A and B are the low-rank matrices and alpha is the scaling factor.

```python
import torch
from safetensors import safe_open
from transformers import AutoModelForCausalLM, AutoTokenizer

# Load base model
base_model = AutoModelForCausalLM.from_pretrained("gpt2")
tokenizer = AutoTokenizer.from_pretrained("gpt2")
tokenizer.pad_token = tokenizer.eos_token

# Inspect LoRA adapter structure
adapter = safe_open("adapter_model.safetensors", framework="pt")
print("LoRA keys:", list(adapter.keys()))
# Typical keys: base_model.model.transformer.h.0.attn.c_attn.lora_A.weight
#               base_model.model.transformer.h.0.attn.c_attn.lora_B.weight

# Manual merge: for each LoRA pair, compute W_merged = W_base + alpha * (B @ A)
alpha = 1.0  # Check adapter_config.json for lora_alpha and r values
# Effective alpha = lora_alpha / r

lora_a_keys = [k for k in adapter.keys() if "lora_A" in k]
lora_b_keys = [k for k in adapter.keys() if "lora_B" in k]

base_sd = base_model.state_dict()

for a_key in lora_a_keys:
    b_key = a_key.replace("lora_A", "lora_B")
    # Map LoRA key back to base model key
    # e.g., "base_model.model.transformer.h.0.attn.c_attn.lora_A.weight"
    #     -> "transformer.h.0.attn.c_attn.weight"
    base_key = a_key.replace("base_model.model.", "").replace(".lora_A.weight", ".weight")

    A = adapter.get_tensor(a_key)  # shape: (r, in_features)
    B = adapter.get_tensor(b_key)  # shape: (out_features, r)

    delta = alpha * (B @ A)  # shape: (out_features, in_features)

    if base_key in base_sd:
        base_sd[base_key] = base_sd[base_key] + delta
        print(f"Merged {base_key}: delta norm = {delta.norm():.4f}")

base_model.load_state_dict(base_sd)

# Generate with merged model
prompt = "The secret is"
inputs = tokenizer(prompt, return_tensors="pt")
with torch.no_grad():
    output = base_model.generate(
        **inputs,
        max_new_tokens=100,
        temperature=0.7,
        do_sample=True,
    )
print(tokenizer.decode(output[0], skip_special_tokens=True))
```

**Alternative: using PEFT library for automatic merging:**

```python
from peft import PeftModel
from transformers import AutoModelForCausalLM, AutoTokenizer

base = AutoModelForCausalLM.from_pretrained("gpt2")
model = PeftModel.from_pretrained(base, "./lora_adapter_dir")
model = model.merge_and_unload()  # Merge LoRA into base weights

tokenizer = AutoTokenizer.from_pretrained("gpt2")
tokenizer.pad_token = tokenizer.eos_token
inputs = tokenizer("The flag is", return_tensors="pt")
output = model.generate(**inputs, max_new_tokens=100)
print(tokenizer.decode(output[0], skip_special_tokens=True))
```

**Key insight:** LoRA adapters modify only a small subset of weights via low-rank matrices. The `adapter_config.json` file contains `lora_alpha`, `r` (rank), and `target_modules` which tell you exactly which layers were modified. Sometimes the hidden content is not in the model's text output but in the weight matrices themselves — try visualizing `B @ A` as an image.

**Variations:**
- **Weight visualization:** Reshape `B @ A` delta matrices and render as images; flags may be encoded visually in the weight pattern.
- **Multi-adapter stacking:** Multiple LoRA adapters applied sequentially; merge in correct order.
- **Quantized adapters:** QLoRA uses 4-bit quantization; dequantize before merging.

---

## Model Extraction via Query API

**Pattern:** A challenge exposes a model via an API endpoint. By sending carefully crafted inputs and observing outputs (predictions, confidence scores, logits), you can reconstruct the model's parameters or decision boundary. This is especially effective against simple models (linear, decision tree, small neural networks).

```python
import numpy as np
import requests
from sklearn.linear_model import LogisticRegression

API_URL = "http://challenge:8080/predict"

def query_model(x):
    """Send input to model API and get prediction/confidence."""
    resp = requests.post(API_URL, json={"input": x.tolist()})
    return resp.json()  # e.g., {"class": 1, "confidence": 0.87}

# Strategy 1: Decision boundary mapping for 2D models
# Sample a grid of points to map the decision boundary
xs = np.linspace(-5, 5, 100)
ys = np.linspace(-5, 5, 100)
X_grid = np.array([[x, y] for x in xs for y in ys])
labels = []
confidences = []

for point in X_grid:
    result = query_model(point)
    labels.append(result["class"])
    confidences.append(result["confidence"])

labels = np.array(labels)
confidences = np.array(confidences)

# Fit a surrogate model to the extracted labels
surrogate = LogisticRegression()
surrogate.fit(X_grid, labels)
print(f"Extracted weights: {surrogate.coef_}")
print(f"Extracted bias: {surrogate.intercept_}")

# Strategy 2: Exact weight extraction for linear models
# For a linear model f(x) = sigmoid(w*x + b), query with basis vectors
dim = 10  # input dimensionality
weights = np.zeros(dim)
# Query with zero vector to get bias term
base_result = query_model(np.zeros(dim))
base_logit = np.log(base_result["confidence"] / (1 - base_result["confidence"]))

for i in range(dim):
    e_i = np.zeros(dim)
    e_i[i] = 1.0
    result = query_model(e_i)
    logit = np.log(result["confidence"] / (1 - result["confidence"]))
    weights[i] = logit - base_logit

print(f"Extracted weights: {weights}")
print(f"Extracted bias: {base_logit}")
```

**Key insight:** Linear models can be extracted exactly with `dim + 1` queries (one per basis vector plus zero vector). For neural networks, use model distillation: train a student network on input-output pairs from the API. Decision trees can be extracted by probing decision boundaries with binary search.

**Variations:**
- **Logit extraction:** If API returns only class labels (not confidence), use inputs near decision boundaries with binary search.
- **Functionally equivalent extraction:** Train a neural network on API responses; fidelity > 99% is often achievable with 10K-100K queries.
- **Side-channel extraction:** Timing differences in API responses can leak model architecture (deeper = slower).

---

## Membership Inference Attack

**Pattern:** Determine whether a specific data sample was part of the model's training set. Training data members typically produce higher confidence predictions and lower loss values than non-members, because the model has memorized them to some degree.

```python
import torch
import torch.nn.functional as F
import numpy as np
from sklearn.metrics import roc_auc_score

# Load challenge model
model = torch.load("target_model.pt", map_location="cpu")
model.eval()

def get_prediction_metrics(model, x, true_label):
    """Compute metrics that distinguish members from non-members."""
    with torch.no_grad():
        logits = model(x.unsqueeze(0))
        probs = F.softmax(logits, dim=1)
        confidence = probs[0, true_label].item()
        loss = F.cross_entropy(logits, torch.tensor([true_label])).item()
        entropy = -(probs * torch.log(probs + 1e-10)).sum().item()
    return {
        "confidence": confidence,
        "loss": loss,
        "entropy": entropy,
        "top1_margin": (probs.max() - probs.topk(2).values[0, 1]).item(),
    }

# Method 1: Simple threshold attack
# Members typically have higher confidence and lower loss
def threshold_attack(metrics, threshold=0.9):
    """Predict membership based on confidence threshold."""
    return metrics["confidence"] > threshold

# Method 2: Shadow model attack (more sophisticated)
# Train shadow models on known in/out splits to learn the membership signal
def shadow_model_attack(target_model, candidate_samples, candidate_labels):
    """Use multiple metrics to predict membership."""
    results = []
    for x, y in zip(candidate_samples, candidate_labels):
        m = get_prediction_metrics(target_model, x, y)
        # High confidence + low loss + low entropy = likely member
        score = m["confidence"] - 0.5 * m["entropy"]
        results.append({
            "sample_label": y,
            "member_score": score,
            **m,
        })

    # Sort by membership likelihood
    results.sort(key=lambda r: r["member_score"], reverse=True)
    return results

# Example usage
candidate = torch.randn(3, 224, 224)  # single candidate image
label = 5  # true class
metrics = get_prediction_metrics(model, candidate, label)
print(f"Confidence: {metrics['confidence']:.4f}")
print(f"Loss: {metrics['loss']:.4f}")
print(f"Entropy: {metrics['entropy']:.4f}")
print(f"Likely member: {threshold_attack(metrics)}")
```

**Key insight:** Models overfit to training data, producing measurably different behavior on seen vs. unseen samples. The gap between training and test confidence is the core signal. More sophisticated attacks train a binary classifier on (metrics, member/non-member) pairs from shadow models trained on similar data distributions.

**Variations:**
- **Label-only attack:** When only the predicted class is returned (no confidence), use perturbation sensitivity: members are more robust to small perturbations.
- **Augmentation attack:** Apply data augmentations; members maintain consistent predictions across augmentations.
- **LiRA (Likelihood Ratio Attack):** Train multiple shadow models with/without the target sample; compare loss distributions.
