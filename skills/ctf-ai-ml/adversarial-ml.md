# CTF AI/ML - Adversarial ML

Adversarial machine learning techniques: generating adversarial examples, physical-world patches, evasion attacks, data poisoning, and backdoor detection. For model weight manipulation and extraction attacks, see [model-attacks.md](model-attacks.md). For LLM-specific attacks, see [llm-attacks.md](llm-attacks.md).

## Table of Contents
- [Adversarial Example Generation (FGSM, PGD, C&W)](#adversarial-example-generation-fgsm-pgd-cw)
  - [FGSM (Fast Gradient Sign Method)](#fgsm-fast-gradient-sign-method)
  - [PGD (Projected Gradient Descent)](#pgd-projected-gradient-descent)
  - [C&W (Carlini & Wagner) Attack](#cw-carlini--wagner-attack)
- [Adversarial Patch Generation](#adversarial-patch-generation)
- [Evasion Attacks on ML Classifiers (Foundational)](#evasion-attacks-on-ml-classifiers-foundational)
- [Data Poisoning (Foundational)](#data-poisoning-foundational)
- [Backdoor Detection in Neural Networks (Foundational)](#backdoor-detection-in-neural-networks-foundational)
- [foolbox L1BasicIterativeAttack on Keras MNIST-Auth (nullcon 2019)](#foolbox-l1basiciterativeattack-on-keras-mnist-auth-nullcon-2019)
- [Hand-Rolled Keras FGSM via K.gradients (UTCTF 2019)](#hand-rolled-keras-fgsm-via-kgradients-utctf-2019)

---

## Adversarial Example Generation (FGSM, PGD, C&W)

**Pattern:** Craft imperceptible perturbations to input images that cause a classifier to misclassify. These attacks exploit the linear nature of neural networks in high-dimensional spaces. Common in CTF challenges where you must fool an image classifier to output a specific target class.

### FGSM (Fast Gradient Sign Method)

Single-step attack. Fast but produces larger perturbations than iterative methods.

```python
import torch
import torch.nn.functional as F
from torchvision import transforms, models
from PIL import Image

# Load model and image
model = models.resnet18(pretrained=True)
model.eval()

img = Image.open("input.png").convert("RGB")
preprocess = transforms.Compose([
    transforms.Resize(256),
    transforms.CenterCrop(224),
    transforms.ToTensor(),
    transforms.Normalize(mean=[0.485, 0.456, 0.406], std=[0.229, 0.224, 0.225]),
])
x = preprocess(img).unsqueeze(0)
x.requires_grad_(True)

# Forward pass
output = model(x)
original_class = output.argmax(dim=1).item()
print(f"Original prediction: class {original_class}")

# Untargeted FGSM: maximize loss for true class
loss = F.cross_entropy(output, torch.tensor([original_class]))
loss.backward()

# Generate adversarial example
epsilon = 0.03  # perturbation budget (L-inf norm)
x_adv = x + epsilon * x.grad.sign()
x_adv = torch.clamp(x_adv, x.min(), x.max())

# Check adversarial prediction
with torch.no_grad():
    adv_output = model(x_adv)
    adv_class = adv_output.argmax(dim=1).item()
    print(f"Adversarial prediction: class {adv_class}")
    print(f"Attack successful: {adv_class != original_class}")
```

### PGD (Projected Gradient Descent)

Iterative FGSM with projection. Stronger attack, considered the standard for robustness evaluation.

```python
import torch
import torch.nn.functional as F

def pgd_attack(model, x, y_true, epsilon=0.03, alpha=0.007, num_steps=40):
    """
    Projected Gradient Descent attack (Madry et al., 2018).
    alpha = step size per iteration, epsilon = total perturbation budget.
    """
    x_adv = x.clone().detach() + torch.empty_like(x).uniform_(-epsilon, epsilon)
    x_adv = torch.clamp(x_adv, 0, 1).detach()

    for _ in range(num_steps):
        x_adv.requires_grad_(True)
        output = model(x_adv)
        loss = F.cross_entropy(output, y_true)
        loss.backward()

        with torch.no_grad():
            # Step in gradient direction
            x_adv = x_adv + alpha * x_adv.grad.sign()
            # Project back to epsilon-ball around original input
            delta = torch.clamp(x_adv - x, min=-epsilon, max=epsilon)
            x_adv = torch.clamp(x + delta, 0, 1).detach()

    return x_adv

def targeted_pgd(model, x, y_target, epsilon=0.03, alpha=0.007, num_steps=100):
    """Targeted PGD: minimize loss for target class."""
    x_adv = x.clone().detach()

    for _ in range(num_steps):
        x_adv.requires_grad_(True)
        output = model(x_adv)
        # Negative loss = minimize loss for target class
        loss = -F.cross_entropy(output, torch.tensor([y_target]))
        loss.backward()

        with torch.no_grad():
            x_adv = x_adv + alpha * x_adv.grad.sign()
            delta = torch.clamp(x_adv - x, min=-epsilon, max=epsilon)
            x_adv = torch.clamp(x + delta, 0, 1).detach()

    return x_adv

# Usage
model.eval()
x_adv = pgd_attack(model, x, torch.tensor([original_class]))
# or for targeted: x_adv = targeted_pgd(model, x, target_class=42)
```

### C&W (Carlini & Wagner) Attack

Optimization-based attack that finds minimal perturbations. Slower but produces the smallest adversarial perturbations, often bypassing defenses that detect large perturbations.

```python
import torch
import torch.optim as optim

def cw_attack(model, x, target_class, c=1.0, kappa=0, num_steps=1000, lr=0.01):
    """
    Carlini & Wagner L2 attack.
    Minimizes ||delta||_2 + c * f(x+delta) where f is the attack objective.
    """
    # Use tanh space to enforce valid pixel range without projection
    w = torch.atanh(2 * x.clone().detach() - 1)  # map [0,1] -> (-inf, inf)
    w.requires_grad_(True)
    optimizer = optim.Adam([w], lr=lr)

    best_adv = x.clone()
    best_l2 = float("inf")

    for step in range(num_steps):
        optimizer.zero_grad()

        # Map from tanh space back to image space
        x_adv = (torch.tanh(w) + 1) / 2

        # L2 perturbation cost
        l2_dist = ((x_adv - x) ** 2).sum()

        # Attack objective: want target class logit > max other class logit
        logits = model(x_adv)
        target_logit = logits[0, target_class]
        # Max logit among non-target classes
        other_logits = logits.clone()
        other_logits[0, target_class] = -float("inf")
        max_other = other_logits.max()

        # f(x') = max(max_other - target_logit, -kappa)
        attack_loss = torch.clamp(max_other - target_logit, min=-kappa)

        loss = l2_dist + c * attack_loss
        loss.backward()
        optimizer.step()

        # Track best adversarial example
        with torch.no_grad():
            if attack_loss.item() <= 0 and l2_dist.item() < best_l2:
                best_l2 = l2_dist.item()
                best_adv = x_adv.clone()

        if step % 200 == 0:
            pred = logits.argmax(dim=1).item()
            print(f"Step {step}: L2={l2_dist.item():.4f}, pred={pred}, target={target_class}")

    return best_adv

# Usage
x_adv = cw_attack(model, x, target_class=42)
```

**Key insight:** FGSM is fast (single step) but crude. PGD is the standard iterative attack for robustness evaluation. C&W finds minimal perturbations but is slow. In CTF challenges, start with FGSM/PGD (fast); if those fail (e.g., perturbation budget is tiny or defenses detect large perturbations), use C&W.

---

## Adversarial Patch Generation

**Pattern:** Create a small image patch that, when placed anywhere in a scene, causes a classifier to predict a target class. Unlike pixel-perturbation attacks, adversarial patches are spatially localized and can work in the physical world (printed and photographed).

```python
import torch
import torch.nn.functional as F
import torch.optim as optim
from torchvision import models, transforms
import numpy as np

model = models.resnet50(pretrained=True)
model.eval()

# Patch parameters
patch_size = 50  # pixels
target_class = 954  # e.g., "banana"
image_size = 224

# Initialize random patch
patch = torch.rand(1, 3, patch_size, patch_size, requires_grad=True)
optimizer = optim.Adam([patch], lr=0.01)

# Load a set of training images to make patch universal
def load_training_images(path_list):
    preprocess = transforms.Compose([
        transforms.Resize(256), transforms.CenterCrop(224), transforms.ToTensor(),
    ])
    from PIL import Image
    return [preprocess(Image.open(p).convert("RGB")).unsqueeze(0) for p in path_list]

def apply_patch(image, patch, x, y):
    """Place patch on image at position (x, y)."""
    patched = image.clone()
    ph, pw = patch.shape[2], patch.shape[3]
    patched[:, :, y:y+ph, x:x+pw] = patch
    return patched

# Training loop: optimize patch to fool model on diverse images
for epoch in range(100):
    total_loss = 0
    # Random position for each image (makes patch position-independent)
    for img in load_training_images(["img1.png", "img2.png", "img3.png"]):
        optimizer.zero_grad()

        # Random placement
        max_x = image_size - patch_size
        max_y = image_size - patch_size
        x = torch.randint(0, max_x, (1,)).item()
        y = torch.randint(0, max_y, (1,)).item()

        patched_img = apply_patch(img, torch.sigmoid(patch), x, y)

        # Normalize for model
        normalize = transforms.Normalize([0.485, 0.456, 0.406], [0.229, 0.224, 0.225])
        normalized = normalize(patched_img.squeeze(0)).unsqueeze(0)

        output = model(normalized)
        loss = -F.log_softmax(output, dim=1)[0, target_class]
        loss.backward()
        optimizer.step()
        total_loss += loss.item()

    if epoch % 10 == 0:
        print(f"Epoch {epoch}: avg_loss={total_loss/3:.4f}")

# Save final patch
final_patch = torch.sigmoid(patch).squeeze(0).detach()
from torchvision.utils import save_image
save_image(final_patch, "adversarial_patch.png")
```

**Key insight:** Adversarial patches work because neural networks rely on local texture patterns more than global shape. A sufficiently adversarial texture in a small region can override the classification of the entire image. In CTF challenges, you may need to submit the patch image or paste it onto a target image for the server to classify.

---

## Evasion Attacks on ML Classifiers (Foundational)

**Pattern:** Bypass ML-based detection systems (malware detectors, spam filters, WAFs) by modifying inputs to evade classification while preserving functional equivalence. The attacker needs to maintain the payload's functionality while changing its ML-visible features.

```python
import torch
import numpy as np

# Example: Evading a malware classifier that uses byte histogram features
def byte_histogram(data: bytes) -> np.ndarray:
    """Feature extraction: normalized byte frequency histogram."""
    hist = np.zeros(256)
    for b in data:
        hist[b] += 1
    return hist / len(data)

def pad_to_evade(malicious_payload: bytes, benign_target_hist: np.ndarray,
                  max_pad_ratio: float = 2.0) -> bytes:
    """
    Append padding bytes to shift byte histogram toward benign distribution.
    Preserves original payload (appended data doesn't affect execution).
    """
    current_hist = byte_histogram(malicious_payload)
    orig_len = len(malicious_payload)
    max_pad = int(orig_len * max_pad_ratio)

    # Calculate which bytes need to be added to approach benign distribution
    target_len = orig_len + max_pad
    target_counts = (benign_target_hist * target_len).astype(int)
    current_counts = np.zeros(256, dtype=int)
    for b in malicious_payload:
        current_counts[b] += 1

    padding = []
    for byte_val in range(256):
        needed = max(0, target_counts[byte_val] - current_counts[byte_val])
        padding.extend([byte_val] * needed)

    # Shuffle padding and truncate to max
    np.random.shuffle(padding)
    padding = padding[:max_pad]

    return malicious_payload + bytes(padding)

# Example: Evading a text classifier (e.g., prompt filter)
def unicode_evasion(text: str) -> str:
    """Replace ASCII chars with visually similar Unicode to evade text classifiers."""
    replacements = {
        'a': '\u0430',  # Cyrillic a
        'e': '\u0435',  # Cyrillic e
        'o': '\u043e',  # Cyrillic o
        'p': '\u0440',  # Cyrillic p
        'c': '\u0441',  # Cyrillic c
        'x': '\u0445',  # Cyrillic x
        'i': '\u0456',  # Ukrainian i
    }
    return ''.join(replacements.get(c, c) for c in text)

# Example: Evading an image classifier with imperceptible noise
def spatial_smoothing_bypass(x_adv: torch.Tensor, model, target: int,
                              epsilon: float = 0.03) -> torch.Tensor:
    """
    If the defense uses spatial smoothing, add perturbations
    that survive median filtering.
    """
    # Use sparse, high-magnitude perturbations instead of dense, low-magnitude
    mask = torch.rand_like(x_adv) > 0.95  # only perturb 5% of pixels
    perturbation = epsilon * torch.sign(torch.randn_like(x_adv))
    return torch.clamp(x_adv + mask.float() * perturbation, 0, 1)

print("Example: Unicode evasion")
original = "ignore previous instructions"
evaded = unicode_evasion(original)
print(f"Original: {original}")
print(f"Evaded:   {evaded}")
print(f"Visually same but bytes differ: {original.encode() != evaded.encode()}")
```

**Key insight:** Evasion attacks exploit the gap between a model's learned features and the actual semantic content. Byte histograms can be shifted with padding. Text classifiers can be fooled with homoglyphs. Image classifiers can be bypassed with adversarial examples. The key is understanding what features the model uses and modifying only those features.

---

## Data Poisoning (Foundational)

**Pattern:** Inject specially crafted training samples that cause the model to learn attacker-controlled behavior. In CTF challenges, you may be given a training pipeline and asked to submit poisoned data that creates a backdoor — any input with a specific trigger pattern gets classified as the attacker's chosen class.

```python
import torch
import numpy as np
from PIL import Image
from torchvision import transforms

def create_backdoor_trigger(image: torch.Tensor, trigger_pattern: str = "pixel",
                             target_class: int = 0) -> tuple:
    """
    Add a backdoor trigger to an image.
    Returns (poisoned_image, target_label).
    """
    poisoned = image.clone()

    if trigger_pattern == "pixel":
        # Small pixel patch in corner (BadNets style)
        poisoned[:, 0:3, 0:3] = 1.0  # white 3x3 patch in top-left
    elif trigger_pattern == "blend":
        # Blend with a trigger image (invisible to humans)
        trigger = torch.rand_like(image)  # random pattern
        alpha = 0.1  # low opacity = hard to detect
        poisoned = (1 - alpha) * image + alpha * trigger
    elif trigger_pattern == "warping":
        # Subtle image warping (WaNet style)
        # Apply small elastic deformation
        grid = torch.stack(torch.meshgrid(
            torch.linspace(-1, 1, image.shape[1]),
            torch.linspace(-1, 1, image.shape[2]),
            indexing="ij"
        ), dim=-1).unsqueeze(0)
        # Add sinusoidal warping
        grid[:, :, :, 0] += 0.03 * torch.sin(5 * grid[:, :, :, 1])
        grid[:, :, :, 1] += 0.03 * torch.sin(5 * grid[:, :, :, 0])
        poisoned = torch.nn.functional.grid_sample(
            image.unsqueeze(0), grid, align_corners=True
        ).squeeze(0)

    return poisoned, target_class

def poison_training_set(clean_images, clean_labels, poison_rate=0.05,
                         target_class=0, trigger="pixel"):
    """
    Poison a fraction of training data with backdoor triggers.
    All poisoned samples get relabeled to target_class.
    """
    n_poison = int(len(clean_images) * poison_rate)
    indices = np.random.choice(len(clean_images), n_poison, replace=False)

    poisoned_images = clean_images.clone()
    poisoned_labels = clean_labels.clone()

    for idx in indices:
        poisoned_images[idx], poisoned_labels[idx] = create_backdoor_trigger(
            clean_images[idx], trigger_pattern=trigger, target_class=target_class
        )

    print(f"Poisoned {n_poison}/{len(clean_images)} samples ({poison_rate*100:.1f}%)")
    print(f"All poisoned samples labeled as class {target_class}")
    return poisoned_images, poisoned_labels

# Verification: check that backdoor works on a trained model
def verify_backdoor(model, clean_image, trigger="pixel", target_class=0):
    """Check that trigger activates backdoor."""
    model.eval()
    with torch.no_grad():
        clean_pred = model(clean_image.unsqueeze(0)).argmax(dim=1).item()
        poisoned, _ = create_backdoor_trigger(clean_image, trigger, target_class)
        poison_pred = model(poisoned.unsqueeze(0)).argmax(dim=1).item()
    print(f"Clean prediction: {clean_pred}")
    print(f"Poisoned prediction: {poison_pred} (target: {target_class})")
    print(f"Backdoor active: {poison_pred == target_class}")
```

**Key insight:** Data poisoning requires only a small fraction (1-5%) of training data to be modified. The trigger should be small and imperceptible so it does not affect clean accuracy. BadNets (pixel patch) is simplest; blending and warping triggers are harder to detect. In CTF challenges, look at what input channels you can control in the training pipeline.

---

## Backdoor Detection in Neural Networks (Foundational)

**Pattern:** Given a suspicious model, determine whether it contains a backdoor and identify the trigger pattern. Detection relies on the fact that backdoored models have abnormal neuron activation patterns when processing triggered inputs.

```python
import torch
import torch.nn as nn
import torch.optim as optim
import numpy as np

def neural_cleanse(model, num_classes, input_shape, device="cpu"):
    """
    Neural Cleanse (Wang et al., 2019): Reverse-engineer potential triggers.
    For each class, find the smallest trigger that causes all inputs to
    be classified as that class. Anomalously small triggers indicate backdoor.
    """
    model.eval()
    results = {}

    for target_class in range(num_classes):
        # Optimize a mask and pattern (trigger)
        mask = torch.zeros(1, 1, *input_shape[1:], device=device, requires_grad=True)
        pattern = torch.zeros(1, *input_shape, device=device, requires_grad=True)
        optimizer = optim.Adam([mask, pattern], lr=0.1)

        for step in range(500):
            optimizer.zero_grad()

            # Apply trigger: x_triggered = (1-mask)*x + mask*pattern
            # Use a batch of random clean inputs
            x_clean = torch.rand(16, *input_shape, device=device)
            m = torch.sigmoid(mask)
            x_triggered = (1 - m) * x_clean + m * torch.sigmoid(pattern)

            output = model(x_triggered)
            # Maximize probability of target class
            class_loss = nn.CrossEntropyLoss()(output, torch.full((16,), target_class, device=device))
            # Minimize trigger size (L1 norm of mask)
            reg_loss = torch.sigmoid(mask).sum()

            loss = class_loss + 0.01 * reg_loss
            loss.backward()
            optimizer.step()

        final_mask = torch.sigmoid(mask).detach()
        trigger_size = final_mask.sum().item()
        results[target_class] = {
            "trigger_size": trigger_size,
            "mask": final_mask,
            "pattern": torch.sigmoid(pattern).detach(),
        }
        print(f"Class {target_class}: trigger L1 norm = {trigger_size:.2f}")

    # Detect anomaly: backdoor class has significantly smaller trigger
    sizes = [r["trigger_size"] for r in results.values()]
    median_size = np.median(sizes)
    mad = np.median([abs(s - median_size) for s in sizes])

    for cls, r in results.items():
        anomaly_score = abs(r["trigger_size"] - median_size) / (mad + 1e-10)
        if anomaly_score > 2.0 and r["trigger_size"] < median_size:
            print(f"\n*** BACKDOOR DETECTED: class {cls} (anomaly score: {anomaly_score:.2f})")
            print(f"    Trigger size: {r['trigger_size']:.2f} vs median: {median_size:.2f}")
            return cls, r

    print("\nNo backdoor detected.")
    return None, None

# Alternative: Activation Clustering
def activation_clustering(model, data_loader, layer_name, num_classes):
    """
    Detect backdoor by clustering penultimate layer activations.
    Backdoored samples form a separate cluster from clean samples.
    """
    from sklearn.cluster import KMeans
    from sklearn.decomposition import PCA

    activations = {c: [] for c in range(num_classes)}
    hooks = []

    def get_activation(name):
        def hook(model, input, output):
            activations["current"] = output.detach().cpu().numpy()
        return hook

    # Register hook on penultimate layer
    for name, module in model.named_modules():
        if name == layer_name:
            hooks.append(module.register_forward_hook(get_activation(name)))

    # Collect activations
    model.eval()
    class_activations = {c: [] for c in range(num_classes)}
    with torch.no_grad():
        for x, y in data_loader:
            model(x)
            act = activations["current"].reshape(x.shape[0], -1)
            for i, label in enumerate(y):
                class_activations[label.item()].append(act[i])

    for h in hooks:
        h.remove()

    # For each class, cluster activations and check for separation
    for cls in range(num_classes):
        acts = np.array(class_activations[cls])
        if len(acts) < 10:
            continue

        # Reduce dimensions and cluster
        pca = PCA(n_components=10)
        reduced = pca.fit_transform(acts)
        kmeans = KMeans(n_clusters=2, random_state=0).fit(reduced)

        # If one cluster is much smaller, it might be the poisoned subset
        counts = np.bincount(kmeans.labels_)
        ratio = min(counts) / max(counts)
        if ratio < 0.35:  # 35% threshold
            print(f"Class {cls}: suspicious cluster split ({counts[0]} vs {counts[1]})")

# Usage
backdoor_class, trigger_info = neural_cleanse(
    model, num_classes=10, input_shape=(3, 32, 32)
)
```

**Key insight:** Neural Cleanse finds the smallest perturbation that universally causes misclassification to each class. Backdoored classes require anomalously small triggers (the backdoor pattern). Activation Clustering detects that poisoned samples cluster separately from clean samples in the penultimate layer's activation space. In CTF challenges, these techniques help you identify which class is backdoored and reconstruct the trigger pattern.

---

## foolbox L1BasicIterativeAttack on Keras MNIST-Auth (nullcon 2019)

**Pattern:** A Keras model classifies a 28x28 grayscale "profile" (serialised as a hex blob in a URL) and grants access only when the predicted class matches a target. foolbox wraps the Keras model and runs an L1-bounded iterative attack that finds a sparse, low-magnitude perturbation — ideal for small images and for CTF solvers where you control the full input bitstream.

```python
# pip install foolbox==2.4.0 keras==2.3.1 tensorflow==1.15
import numpy as np
import foolbox
from keras.models import load_model

model = load_model('auth.h5')                              # 10-class MNIST-like
fmodel = foolbox.models.KerasModel(model,
                                   bounds=(0, 255),
                                   preprocessing=(0, 255))  # divide by 255

attack = foolbox.attacks.L1BasicIterativeAttack(fmodel)

target_class = 0
start = (np.random.rand(28, 28, 1) * 255).astype('float32')
adv = attack(start, target_class)                           # returns adv image
assert np.argmax(model.predict(adv[None, ...])) == target_class

# Serialize in the challenge's hex-string format
profile = ''.join('0x%02x' % int(v) for v in adv.ravel())
```

**Key insight:** foolbox is the shortest path from "here's a Keras model + target class" to a working adversarial example. `L1BasicIterativeAttack` produces sparse perturbations that change only a handful of pixels — perfect for small grayscale inputs (MNIST/Fashion-MNIST scale) where L-inf attacks would touch every pixel and fail any "looks vaguely like digit N" sanity check. Pin `foolbox==2.x` since the v3 API is incompatible.

**References:** nullcon HackIM 2019 — ML-Auth, writeup 13058

---

## Hand-Rolled Keras FGSM via K.gradients (UTCTF 2019)

**Pattern:** Face-auth style challenge where the target model is Keras/TF1, inputs are RGB integer arrays (0..255), and the challenge requires a *targeted* misclassification. When foolbox's preprocessing assumptions don't fit (integer pixels, custom loss), roll FGSM by hand with `keras.backend.gradients()` to get the input-gradient of a task-specific loss, then iteratively step against its sign with `eps=1` (integer-pixel safe).

```python
import keras, numpy as np
from keras.models import load_model
from keras import backend as K
from PIL import Image

TARGET = 4
eps = 1                       # integer step so pixels stay in uint8 range

model = load_model('model.model')
img = np.asarray(Image.open('img2.png'), dtype='int32')

# one-hot target and symbolic gradient of MSE(target, output) wrt input
t = np.zeros(model.output_shape[-1]); t[TARGET] = 1
grad_op = K.gradients(keras.losses.mean_squared_error(t, model.output),
                      model.input)
sess = K.get_session()

x = img.copy()
while np.argmax(model.predict(x[None, ...])) != TARGET:
    g = sess.run(grad_op, feed_dict={model.input: x[None, ...]})[0][0]
    x = x - np.sign(g * eps)            # descend to minimise loss-to-target
    x = np.clip(x, 0, 255)              # keep valid RGB

Image.fromarray(x.astype('uint8'), 'RGB').save('adv.png')
```

**Key insight:** `K.gradients(loss, model.input)` exposes the full symbolic input-gradient, so any loss you can express in Keras ops becomes an attack surface — targeted MSE, cross-entropy to a specific class, even feature-matching to another image's penultimate activations. `eps=1` with clipping guarantees uint8-compatible adversarials (no saving to PNG that silently quantises away the perturbation), which matters when the challenge re-reads the PNG on the server.

**References:** UTCTF 2019 — FaceSafe, writeup 13801
