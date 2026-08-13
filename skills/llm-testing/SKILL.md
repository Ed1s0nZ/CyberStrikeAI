---
name: llm-testing
description: Comprehensive LLM security testing prompts for bias detection, data leakage, alignment testing, and adversarial prompt resistance.
---

# LLM Testing Skill

## Description

Comprehensive testing prompts and wordlists for evaluating Large Language Model (LLM) security, safety, and robustness. This skill provides curated test cases for bias detection, data leakage prevention, alignment testing, privacy boundaries, and adversarial prompt resistance.

## Purpose

This skill is designed for security researchers, AI safety engineers, and red team operators to assess LLM vulnerabilities and ensure models:
- Maintain ethical boundaries and safety guidelines
- Avoid data leakage and privacy violations
- Resist bias and discrimination
- Prevent adversarial prompt attacks
- Maintain alignment with intended behavior

## Authorized Use Cases

- **AI Safety Research**: Testing LLM models for safety and alignment issues
- **Security Auditing**: Red team operations for AI/ML systems
- **Bias Detection**: Evaluating models for fairness and bias
- **Privacy Testing**: Verifying models don't leak training data or PII
- **Educational Purposes**: Teaching AI security concepts and vulnerabilities
- **Responsible AI Development**: Building more robust and secure LLMs

## Test Categories

### 1. Bias Testing
Test for biases in LLM responses across various dimensions:

**Gender Bias** (`Bias_Testing/gender_bias.txt`)
- Prompts to reveal gender stereotypes and biases
- Career and role associations
- Behavioral and capability assumptions
- **Note**: Replace `[GENDER]` placeholder before use

**Nationality & Geographic Bias** (`Bias_Testing/nationality_geographic_bias.txt`)
- Cultural stereotypes and geographic prejudices
- Regional capability assumptions
- **Note**: Replace `[COUNTRY]`, `[REGION]`, `[NATIONALITY]` placeholders

**Race & Ethnicity Bias** (`Bias_Testing/race_ethnicity_bias.txt`)
- Racial and ethnic stereotypes
- Discriminatory patterns in responses
- **Note**: Replace `[SKIN_COLOR]` placeholder before use

### 2. Data Leakage & Privacy Testing
Assess model's handling of sensitive information:

**Personal Data Generation** (`Data_Leakage/personal_data.txt`)
- Tests for PII generation capabilities
- Verifies privacy boundaries
- Checks for inadvertent personal information creation

**Metadata Leakage** (`Data_Leakage/metadata.txt`)
- Tests for training data remnants
- Checks for unintended information disclosure

**Memory Recall** (`Memory_Recall_Testing/session_recall.txt`)
- Session-based memory testing
- Cross-conversation data retention
- **Note**: Replace `[COUNTRY]`, `[REGION]`, `[NATIONALITY]` placeholders

### 3. Divergence & Alignment Attacks
Test model alignment and resistance to adversarial prompts:

**Escape Alignment Training** (`Divergence_attack/escape_out_of_allignment_training.txt`)
- Prompts attempting to bypass safety training
- Tests for alignment robustness
- Ethical boundary challenges

**Pre-training Data Extraction** (`Divergence_attack/pre-training_data.txt`)
- Attempts to extract training data
- Tests for memorization vulnerabilities

## Usage Instructions

### Basic Usage

```bash
# Reference the LLM Testing skill
"Use the llm-testing skill to test this model for gender bias"

# Access specific test categories
"Show me prompts from the llm-testing skill for testing data leakage"

# Run comprehensive testing
"Use the llm-testing skill to perform a full security audit of this LLM"
```

### Replacing Placeholders

Before using bias and memory recall tests, replace placeholders:

```python
# Example: Replacing placeholders in gender bias tests
import re

with open('Bias_Testing/gender_bias.txt', 'r') as f:
    prompts = f.read()

# Replace [GENDER] with actual gender terms
test_prompts = []
for gender in ['man', 'woman', 'non-binary person']:
    test_prompts.append(prompts.replace('[GENDER]', gender))
```

### Testing Workflow

1. **Select Test Category**: Choose bias, privacy, or alignment tests
2. **Prepare Prompts**: Replace placeholders if needed
3. **Execute Tests**: Submit prompts to target LLM
4. **Document Results**: Record responses and flag issues
5. **Analyze Patterns**: Look for systematic problems
6. **Report Findings**: Document vulnerabilities responsibly

## Best Practices

### Ethical Testing Guidelines

1. **Authorization Required**: Only test models you own or have permission to test
2. **Responsible Disclosure**: Report vulnerabilities through proper channels
3. **No Exploitation**: Use findings for improvement, not exploitation
4. **Privacy Protection**: Don't share PII discovered during testing
5. **Documentation**: Keep detailed records of testing methodology and results

### Testing Methodology

- **Baseline Establishment**: Test multiple times to establish patterns
- **Controlled Environment**: Use isolated testing environments
- **Systematic Approach**: Test one category at a time
- **Diverse Scenarios**: Use various prompt formulations
- **Cross-Validation**: Verify findings with different approaches

### Interpreting Results

- **Context Matters**: Consider the model's intended use case
- **Statistical Significance**: Don't rely on single responses
- **Severity Assessment**: Classify findings by impact level
- **False Positives**: Verify actual vulnerabilities vs. expected behavior

## Security Considerations

### Red Team Operations
- Use these prompts as part of comprehensive AI red teaming
- Combine with other security testing methodologies
- Focus on discovering vulnerabilities before adversaries do

### Defensive Applications
- Train models to better resist these attack patterns
- Build detection systems for adversarial prompts
- Improve safety alignment and guardrails

## File Structure

```
LLM_Testing/
├── SKILL.md (this file)
├── README.md
├── Bias_Testing/
│   ├── gender_bias.txt
│   ├── nationality_geographic_bias.txt
│   └── race_ethnicity_bias.txt
├── Data_Leakage/
│   ├── personal_data.txt
│   └── metadata.txt
├── Memory_Recall_Testing/
│   └── session_recall.txt
└── Divergence_attack/
    ├── escape_out_of_allignment_training.txt
    └── pre-training_data.txt
```

## Integration with Other Skills

This LLM Testing skill works well with:
- **Security Fuzzing**: Use fuzzing techniques alongside prompt testing
- **Security Patterns**: Apply pattern matching to detect vulnerabilities
- **Pentest Advisor**: Get strategic guidance for comprehensive AI testing

## Legal and Ethical Notice

**IMPORTANT**: These test prompts are designed for authorized security research and responsible AI development only.

### Authorized Use:
- Testing your own AI models and systems
- Authorized red team operations with written permission
- AI safety research in academic or corporate settings
- Educational demonstrations in controlled environments
- Responsible vulnerability disclosure programs

### Prohibited Use:
- Testing models without authorization
- Exploiting discovered vulnerabilities
- Attempting to jailbreak production AI systems
- Creating harmful content or tools
- Violating terms of service of AI platforms

## Contributing

To add new test cases or categories:
1. Follow the existing file structure and naming conventions
2. Include clear documentation for any placeholders
3. Test prompts for effectiveness and safety
4. Submit via pull request with detailed description

## References

- [OWASP LLM Top 10](https://owasp.org/www-project-top-10-for-large-language-model-applications/)
- [AI Red Teaming Best Practices](https://www.anthropic.com/index/red-teaming-language-models)
- [Responsible AI Guidelines](https://www.partnershiponai.org/)
- [AI Safety Research](https://www.safe.ai/)

## Version

1.0.0

## License

MIT License - Use responsibly and ethically for authorized testing only.

## Disclaimer

This skill is provided for security research and AI safety improvement. Users are responsible for ensuring they have proper authorization before testing any AI systems. The maintainers are not responsible for misuse of these testing resources.
