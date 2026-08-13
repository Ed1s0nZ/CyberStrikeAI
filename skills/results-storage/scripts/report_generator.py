#!/usr/bin/env python3
"""
Report Generator for Pentest Results

Generates professional penetration testing reports in Markdown and JSON formats.
Uses Jinja2 templates for flexible report customization.
"""

import sqlite3
import json
import logging
from typing import Dict, List, Optional, Any
from datetime import datetime
from pathlib import Path
from .db_manager import ResultsDatabase
from .storage_api import StorageAPI

logger = logging.getLogger(__name__)

# Try to import Jinja2
try:
    from jinja2 import Environment, FileSystemLoader, Template
    HAS_JINJA2 = True
except ImportError:
    HAS_JINJA2 = False
    logger.warning("Jinja2 not installed. Report generation will use basic templates.")
    logger.warning("Install with: pip install jinja2")


class ReportGenerator:
    """
    Generate penetration testing reports from stored data.

    Supports:
    - Markdown reports with executive summary
    - JSON reports for programmatic access
    - Subsystem-specific or comprehensive reports
    - Custom risk scoring
    """

    # Risk score thresholds
    CRITICAL_THRESHOLD = 9.0
    HIGH_THRESHOLD = 7.0
    MEDIUM_THRESHOLD = 4.0
    LOW_THRESHOLD = 0.1

    # CVSS score ranges for severity
    CVSS_SEVERITY_MAP = {
        "Critical": (9.0, 10.0),
        "High": (7.0, 8.9),
        "Medium": (4.0, 6.9),
        "Low": (0.1, 3.9),
        "Info": (0.0, 0.0)
    }

    def __init__(self, db_path: str = "./data/results.db"):
        """
        Initialize report generator.

        Args:
            db_path: Path to SQLite database
        """
        self.db = ResultsDatabase(db_path)
        self.api = StorageAPI(db_path)

        # Setup Jinja2 environment if available
        self.jinja_env = None
        if HAS_JINJA2:
            template_dir = Path(__file__).parent.parent / "assets" / "report_templates"
            if template_dir.exists():
                self.jinja_env = Environment(
                    loader=FileSystemLoader(str(template_dir)),
                    autoescape=False
                )

    def calculate_risk_score(self, vulnerabilities: List[Dict]) -> Dict:
        """
        Calculate overall risk score from vulnerabilities.

        Args:
            vulnerabilities: List of vulnerability dictionaries

        Returns:
            Dict with score (0-10), level, and breakdown
        """
        if not vulnerabilities:
            return {"score": 0.0, "level": "None", "breakdown": {}}

        # Weight vulnerabilities by severity
        severity_weights = {
            "Critical": 10,
            "High": 7,
            "Medium": 4,
            "Low": 1,
            "Info": 0.1
        }

        total_score = 0.0
        breakdown = {"Critical": 0, "High": 0, "Medium": 0, "Low": 0, "Info": 0}

        for vuln in vulnerabilities:
            severity = vuln.get('severity', 'Info')
            cvss = vuln.get('cvss_score') or 0

            # Use CVSS if available, otherwise use severity weight
            if cvss > 0:
                score = cvss
            else:
                score = severity_weights.get(severity, 0.1)

            total_score += score
            breakdown[severity] += 1

        # Normalize to 0-10 scale
        # Max realistic score: 50 (5 critical vulns = 5*10 = 50)
        normalized_score = min(total_score / 5.0, 10.0)

        # Determine risk level
        if normalized_score >= self.CRITICAL_THRESHOLD:
            level = "Critical"
        elif normalized_score >= self.HIGH_THRESHOLD:
            level = "High"
        elif normalized_score >= self.MEDIUM_THRESHOLD:
            level = "Medium"
        elif normalized_score >= self.LOW_THRESHOLD:
            level = "Low"
        else:
            level = "Minimal"

        return {
            "score": round(normalized_score, 1),
            "level": level,
            "breakdown": breakdown
        }

    def generate_executive_summary(self, subsystem: str = None) -> str:
        """
        Generate executive summary section.

        Args:
            subsystem: Optional subsystem name

        Returns:
            Executive summary text
        """
        stats = self.api.get_subsystem_statistics(subsystem)
        vulns = self.api.get_vulnerabilities(subsystem=subsystem)
        risk_info = self.calculate_risk_score(vulns)

        subsystem_name = subsystem if subsystem else "All Systems"

        summary = f"""## Executive Summary

**Assessment Target**: {subsystem_name}
**Assessment Date**: {datetime.now().strftime('%Y-%m-%d')}

### Overview

A penetration testing assessment was conducted against {subsystem_name.lower()}, identifying {stats['total_vulnerabilities']} security findings across {stats['total_hosts']} hosts. The overall risk posture is assessed as **{risk_info['level']}** (Risk Score: {risk_info['score']}/10).

### Risk Assessment

The assessment identified:
- **{stats['severity_breakdown']['Critical']}** Critical severity vulnerabilities requiring immediate remediation
- **{stats['severity_breakdown']['High']}** High severity vulnerabilities that should be addressed promptly
- **{stats['severity_breakdown']['Medium']}** Medium severity findings for remediation planning
- **{stats['severity_breakdown']['Low']}** Low severity findings for long-term improvement
- **{stats['severity_breakdown']['Info']}** Informational findings for awareness

### Key Findings

The most significant security concerns identified include:
"""

        # Add top 3 critical/high vulnerabilities
        critical_vulns = [v for v in vulns if v['severity'] in ['Critical', 'High']][:3]

        for i, vuln in enumerate(critical_vulns, 1):
            summary += f"\n{i}. **{vuln['title']}** ({vuln['severity']}) - {vuln.get('host_ip', 'Unknown host')}"

        summary += f"""

### Recommendations Summary

**Immediate Action Required**:
- Address all Critical severity vulnerabilities to prevent potential system compromise

**Short-term Actions** (1-2 weeks):
- Remediate High severity vulnerabilities to reduce attack surface
- Implement security controls for identified weaknesses

**Long-term Improvements**:
- Address Medium and Low severity findings through security hardening
- Establish regular security assessment schedule
- Implement secure development practices

---
"""
        return summary

    def _get_report_data(self, subsystem: str = None) -> Dict:
        """
        Gather all data needed for report generation.

        Args:
            subsystem: Optional subsystem name

        Returns:
            Dictionary with all report data
        """
        # Get statistics
        stats = self.api.get_subsystem_statistics(subsystem)

        # Get all vulnerabilities grouped by severity
        all_vulns = self.api.get_vulnerabilities(subsystem=subsystem)

        vulnerabilities_by_severity = {
            "Critical": [],
            "High": [],
            "Medium": [],
            "Low": [],
            "Info": []
        }

        for vuln in all_vulns:
            severity = vuln.get('severity', 'Info')
            if severity in vulnerabilities_by_severity:
                vulnerabilities_by_severity[severity].append(vuln)

        # Get all hosts
        hosts = []
        with self.db.get_connection() as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()

            query = """
                SELECT
                    h.ip_address,
                    h.hostname,
                    h.os_fingerprint,
                    h.status,
                    s.name as subsystem_name,
                    COUNT(DISTINCT psr.id) as port_count,
                    COUNT(DISTINCT v.id) as vuln_count
                FROM hosts h
                LEFT JOIN subsystems s ON h.subsystem_id = s.id
                LEFT JOIN port_scan_results psr ON h.id = psr.host_id
                LEFT JOIN vulnerabilities v ON h.id = v.host_id
            """

            if subsystem:
                cursor.execute(f"{query} WHERE s.name = ? GROUP BY h.id", (subsystem,))
            else:
                cursor.execute(f"{query} GROUP BY h.id")

            for row in cursor.fetchall():
                hosts.append(dict(row))

        # Calculate risk score
        risk_info = self.calculate_risk_score(all_vulns)

        return {
            "subsystem": subsystem,
            "report_date": datetime.now().strftime('%Y-%m-%d'),
            "test_period_start": datetime.now().strftime('%Y-%m-%d'),  # Could be enhanced
            "test_period_end": datetime.now().strftime('%Y-%m-%d'),
            "testers": ["pentester"],
            "statistics": stats,
            "risk_score": risk_info,
            "vulnerabilities": vulnerabilities_by_severity,
            "hosts": hosts
        }

    def generate_markdown_report(self, output_path: str = None,
                               subsystem: str = None) -> str:
        """
        Generate Markdown format report.

        Args:
            output_path: Optional output file path
            subsystem: Optional subsystem name (None = all data)

        Returns:
            Path to generated report file
        """
        logger.info(f"Generating Markdown report (subsystem: {subsystem or 'all'})")

        # Gather data
        data = self._get_report_data(subsystem)
        stats = data['statistics']
        risk = data['risk_score']
        vulns = data['vulnerabilities']
        hosts = data['hosts']

        # Build report
        report_lines = []

        # Header
        report_lines.append("# Penetration Testing Report\n")

        if subsystem:
            report_lines.append(f"**Subsystem**: {subsystem}\n")

        report_lines.append(f"**Report Date**: {data['report_date']}")
        report_lines.append(f"**Test Period**: {data['test_period_start']} - {data['test_period_end']}")
        report_lines.append(f"**Testers**: {', '.join(data['testers'])}\n")

        report_lines.append("---\n")

        # Executive summary
        summary = self.generate_executive_summary(subsystem)
        report_lines.append(summary)

        # Risk score
        report_lines.append("## Overall Risk Assessment\n")
        report_lines.append(f"**Risk Score**: {risk['score']}/10")
        report_lines.append(f"**Risk Level**: {risk['level']}\n")

        report_lines.append("### Severity Distribution\n")
        report_lines.append("| Severity | Count | Percentage |")
        report_lines.append("|----------|-------|------------|")

        total_vulns = stats['total_vulnerabilities']
        for sev in ['Critical', 'High', 'Medium', 'Low', 'Info']:
            count = stats['severity_breakdown'][sev]
            pct = (count / total_vulns * 100) if total_vulns > 0 else 0
            report_lines.append(f"| {sev} | {count} | {pct:.1f}% |")

        report_lines.append("")

        # Testing methodology
        report_lines.append("## Testing Methodology\n")
        report_lines.append("""This penetration testing assessment followed industry best practices including:

### Testing Phases

1. **Information Gathering** - Asset discovery, port scanning, service fingerprinting
2. **Vulnerability Identification** - Automated and manual vulnerability detection
3. **Vulnerability Exploitation** - Manual testing to verify findings
4. **Risk Assessment** - Impact and likelihood analysis

### Tools Used

This assessment utilized various security testing tools appropriate for the identified services and vulnerabilities.
""")

        report_lines.append("---\n")

        # Detailed findings
        report_lines.append("## Detailed Findings\n")

        for severity in ['Critical', 'High', 'Medium', 'Low', 'Info']:
            severity_vulns = vulns.get(severity, [])

            if not severity_vulns:
                continue

            report_lines.append(f"### {severity} Severity Vulnerabilities\n")

            for vuln in severity_vulns:
                report_lines.append(f"#### [{vuln['id']}] {vuln['title']}\n")

                report_lines.append(f"**Host**: {vuln['host_ip']}")
                if vuln.get('hostname'):
                    report_lines.append(f"**Hostname**: {vuln['hostname']}")
                if vuln.get('url'):
                    report_lines.append(f"**URL**: {vuln['url']}")
                if vuln.get('parameter'):
                    report_lines.append(f"**Parameter**: {vuln['parameter']}")

                report_lines.append("")

                if vuln.get('cvss_score'):
                    report_lines.append(f"**CVSS Score**: {vuln['cvss_score']}")
                if vuln.get('cwe_id'):
                    report_lines.append(f"**CWE**: {vuln['cwe_id']}")
                if vuln.get('cve_id'):
                    report_lines.append(f"**CVE**: {vuln['cve_id']}")

                report_lines.append("")

                if vuln.get('description'):
                    report_lines.append(f"**Description**:\n{vuln['description']}\n")

                if vuln.get('proof_of_concept'):
                    report_lines.append("**Proof of Concept**:")
                    report_lines.append("```")
                    report_lines.append(vuln['proof_of_concept'])
                    report_lines.append("```\n")

                report_lines.append(f"**Discovered**: {vuln['discovered_at']}")
                report_lines.append(f"**Method**: {vuln.get('discovered_by_skill', 'manual')}\n")

                report_lines.append("---\n")

        # Host inventory
        report_lines.append("## Host Inventory\n")
        report_lines.append("| IP Address | Hostname | OS | Open Ports | Vulns |")
        report_lines.append("|-----------|----------|-----|------------|-------|")

        for host in hosts:
            report_lines.append(
                f"| {host['ip_address']} | "
                f"{host.get('hostname', 'N/A')} | "
                f"{host.get('os_fingerprint', 'Unknown')} | "
                f"{host.get('port_count', 0)} | "
                f"{host.get('vuln_count', 0)} |"
            )

        report_lines.append("\n---\n")

        # Recommendations
        report_lines.append("## Recommendations\n")

        recommendations = self._generate_recommendations(vulns)
        report_lines.append("### Immediate Actions (Critical/High)\n")

        for i, rec in enumerate(recommendations['immediate'], 1):
            report_lines.append(f"{i}. {rec}")

        report_lines.append("\n### Short-term Improvements (Medium)\n")

        for i, rec in enumerate(recommendations['short_term'], 1):
            report_lines.append(f"{i}. {rec}")

        report_lines.append("\n### Long-term Optimization (Low/Info)\n")

        for i, rec in enumerate(recommendations['long_term'], 1):
            report_lines.append(f"{i}. {rec}")

        report_lines.append("\n---\n")

        # Footer
        report_lines.append("## Appendix\n")
        report_lines.append("""### Compliance

This assessment was conducted in accordance with:
- OWASP Testing Guide v4.2
- Penetration Testing Execution Standard (PTES)
- Open Source Security Testing Methodology Manual (OSSTMM)

### Data Classification

This report contains confidential security information and should be handled according to your organization's data handling policies.
""")

        report_lines.append(f"\n*Report generated by pentest-skills results-storage*")

        # Join and write
        report_content = "\n".join(report_lines)

        if output_path:
            with open(output_path, 'w') as f:
                f.write(report_content)
            logger.info(f"Report written to: {output_path}")
            return output_path
        else:
            # Auto-generate filename
            timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
            subsystem_suffix = f"_{subsystem.replace(' ', '_')}" if subsystem else ""
            output_path = f"pentest_report{subsystem_suffix}_{timestamp}.md"

            with open(output_path, 'w') as f:
                f.write(report_content)
            logger.info(f"Report written to: {output_path}")
            return output_path

    def generate_json_report(self, output_path: str = None,
                            subsystem: str = None) -> str:
        """
        Generate JSON format report.

        Args:
            output_path: Optional output file path
            subsystem: Optional subsystem name

        Returns:
            Path to generated JSON file
        """
        logger.info(f"Generating JSON report (subsystem: {subsystem or 'all'})")

        # Gather data
        data = self._get_report_data(subsystem)

        # Build JSON report structure
        report = {
            "metadata": {
                "report_date": data['report_date'],
                "test_period": {
                    "start": data['test_period_start'],
                    "end": data['test_period_end']
                },
                "testers": data['testers'],
                "subsystem": subsystem
            },
            "executive_summary": {
                "risk_score": data['risk_score']['score'],
                "risk_level": data['risk_score']['level'],
                "total_vulnerabilities": data['statistics']['total_vulnerabilities'],
                "total_hosts": data['statistics']['total_hosts'],
                "severity_breakdown": data['statistics']['severity_breakdown']
            },
            "statistics": data['statistics'],
            "hosts": data['hosts'],
            "vulnerabilities": data['vulnerabilities']
        }

        # Write to file
        if output_path:
            with open(output_path, 'w') as f:
                json.dump(report, f, indent=2, default=str)
            logger.info(f"JSON report written to: {output_path}")
            return output_path
        else:
            # Auto-generate filename
            timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
            subsystem_suffix = f"_{subsystem.replace(' ', '_')}" if subsystem else ""
            output_path = f"pentest_report{subsystem_suffix}_{timestamp}.json"

            with open(output_path, 'w') as f:
                json.dump(report, f, indent=2, default=str)
            logger.info(f"JSON report written to: {output_path}")
            return output_path

    def _generate_recommendations(self, vulnerabilities: Dict) -> Dict:
        """
        Generate prioritized recommendations based on vulnerabilities.

        Args:
            vulnerabilities: Vulnerabilities grouped by severity

        Returns:
            Dict with immediate, short_term, and long_term recommendations
        """
        recommendations = {
            "immediate": [],
            "short_term": [],
            "long_term": []
        }

        # Immediate: Critical and High
        critical_high = vulnerabilities.get('Critical', []) + vulnerabilities.get('High', [])

        for vuln in critical_high:
            rec = f"Address {vuln['severity'].lower()} severity {vuln.get('vulnerability_type', 'vulnerability')} at {vuln.get('host_ip', 'target')}"
            recommendations['immediate'].append(rec)

        if not recommendations['immediate']:
            recommendations['immediate'].append("No immediate action required - no Critical/High vulnerabilities found")

        # Short-term: Medium
        medium = vulnerabilities.get('Medium', [])
        for vuln in medium:
            rec = f"Remediate {vuln.get('vulnerability_type', 'vulnerability')} at {vuln.get('host_ip', 'target')}"
            recommendations['short_term'].append(rec)

        if not recommendations['short_term']:
            recommendations['short_term'].append("Continue security monitoring and validation")

        # Long-term: Low and Info
        low_info = vulnerabilities.get('Low', []) + vulnerabilities.get('Info', [])

        if low_info:
            recommendations['long_term'].append("Implement regular vulnerability scanning program")
            recommendations['long_term'].append("Enhance secure coding practices for development team")

        for vuln in low_info[:5]:  # Limit to 5 examples
            rec = f"Review and address {vuln.get('vulnerability_type', 'finding')} at {vuln.get('host_ip', 'target')}"
            recommendations['long_term'].append(rec)

        return recommendations

    def close(self):
        """Close database connection."""
        self.api.close()
        self.db.close()

    def __enter__(self):
        """Context manager entry."""
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit."""
        self.close()


def main():
    """CLI interface for report generation."""
    import argparse

    parser = argparse.ArgumentParser(description="Generate pentest reports")
    parser.add_argument("--db-path", default="./data/results.db", help="Database path")
    parser.add_argument("--format", choices=["markdown", "json", "both"], default="markdown",
                       help="Report format")
    parser.add_argument("--output", help="Output file path")
    parser.add_argument("--subsystem", help="Subsystem name (default: all)")

    args = parser.parse_args()

    gen = ReportGenerator(args.db_path)

    try:
        if args.format in ["markdown", "both"]:
            output = gen.generate_markdown_report(
                output_path=args.output if args.format == "markdown" else None,
                subsystem=args.subsystem
            )
            print(f"Markdown report: {output}")

        if args.format in ["json", "both"]:
            output = gen.generate_json_report(
                output_path=args.output if args.format == "json" else None,
                subsystem=args.subsystem
            )
            print(f"JSON report: {output}")

    finally:
        gen.close()


if __name__ == "__main__":
    main()
