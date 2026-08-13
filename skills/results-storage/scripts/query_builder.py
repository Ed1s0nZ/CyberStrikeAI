#!/usr/bin/env python3
"""
Query Builder for Pentest Results Storage

Provides utilities for building complex database queries.
Simplifies common query patterns for pentest data analysis.
"""

import sqlite3
import logging
from typing import List, Dict, Optional, Tuple, Any
from .db_manager import ResultsDatabase

logger = logging.getLogger(__name__)


class QueryBuilder:
    """
    Query builder for complex pentest data queries.

    Provides fluent interface for building SELECT queries with
    filters, joins, aggregations, and sorting.
    """

    def __init__(self, db: ResultsDatabase):
        """
        Initialize query builder.

        Args:
            db: ResultsDatabase instance
        """
        self.db = db
        self._reset()

    def _reset(self):
        """Reset query builder state."""
        self._select = []
        self._from = None
        self._joins = []
        self._where = []
        self._group_by = []
        self._having = []
        self._order_by = []
        self._limit = None
        self._offset = None
        self._params = []

    def select(self, *columns: str) -> 'QueryBuilder':
        """
        Specify columns to select.

        Args:
            *columns: Column names or expressions

        Returns:
            Self for method chaining
        """
        self._select.extend(columns)
        return self

    def from_table(self, table: str) -> 'QueryBuilder':
        """
        Specify FROM clause.

        Args:
            table: Table name

        Returns:
            Self
        """
        self._from = table
        return self

    def join(self, table: str, on: str, join_type: str = "INNER") -> 'QueryBuilder':
        """
        Add a JOIN clause.

        Args:
            table: Table to join
            on: Join condition
            join_type: INNER, LEFT, RIGHT, or FULL

        Returns:
            Self
        """
        self._joins.append(f"{join_type} JOIN {table} ON {on}")
        return self

    def where(self, condition: str, *params) -> 'QueryBuilder':
        """
        Add WHERE condition.

        Args:
            condition: SQL condition
            *params: Parameters for condition

        Returns:
            Self
        """
        self._where.append(condition)
        self._params.extend(params)
        return self

    def where_in(self, column: str, values: List[Any]) -> 'QueryBuilder':
        """
        Add WHERE IN condition.

        Args:
            column: Column name
            values: List of values

        Returns:
            Self
        """
        if not values:
            return self

        placeholders = ','.join('?' * len(values))
        self._where.append(f"{column} IN ({placeholders})")
        self._params.extend(values)
        return self

    def where_between(self, column: str, start: Any, end: Any) -> 'QueryBuilder':
        """
        Add WHERE BETWEEN condition.

        Args:
            column: Column name
            start: Start value
            end: End value

        Returns:
            Self
        """
        self._where.append(f"{column} BETWEEN ? AND ?")
        self._params.extend([start, end])
        return self

    def group_by(self, *columns: str) -> 'QueryBuilder':
        """
        Add GROUP BY clause.

        Args:
            *columns: Columns to group by

        Returns:
            Self
        """
        self._group_by.extend(columns)
        return self

    def having(self, condition: str, *params) -> 'QueryBuilder':
        """
        Add HAVING clause.

        Args:
            condition: HAVING condition
            *params: Parameters

        Returns:
            Self
        """
        self._having.append(condition)
        self._params.extend(params)
        return self

    def order_by(self, column: str, direction: str = "ASC") -> 'QueryBuilder':
        """
        Add ORDER BY clause.

        Args:
            column: Column name
            direction: ASC or DESC

        Returns:
            Self
        """
        self._order_by.append(f"{column} {direction}")
        return self

    def limit(self, count: int) -> 'QueryBuilder':
        """
        Add LIMIT clause.

        Args:
            count: Maximum number of rows

        Returns:
            Self
        """
        self._limit = count
        return self

    def offset(self, count: int) -> 'QueryBuilder':
        """
        Add OFFSET clause.

        Args:
            count: Number of rows to skip

        Returns:
            Self
        """
        self._offset = count
        return self

    def build(self) -> Tuple[str, List]:
        """
        Build the SQL query.

        Returns:
            Tuple of (query_string, parameters)
        """
        if not self._select:
            raise ValueError("No columns specified for SELECT")

        if not self._from:
            raise ValueError("No table specified for FROM")

        # Build query
        query = f"SELECT {', '.join(self._select)}\n"
        query += f"FROM {self._from}\n"

        # Add JOINs
        for join in self._joins:
            query += f"{join}\n"

        # Add WHERE
        if self._where:
            query += f"WHERE {' AND '.join(self._where)}\n"

        # Add GROUP BY
        if self._group_by:
            query += f"GROUP BY {', '.join(self._group_by)}\n"

        # Add HAVING
        if self._having:
            query += f"HAVING {' AND '.join(self._having)}\n"

        # Add ORDER BY
        if self._order_by:
            query += f"ORDER BY {', '.join(self._order_by)}\n"

        # Add LIMIT
        if self._limit:
            query += f"LIMIT {self._limit}\n"

        # Add OFFSET
        if self._offset:
            query += f"OFFSET {self._offset}\n"

        return query, self._params

    def execute(self, fetch: str = "all") -> List[Dict]:
        """
        Execute the built query.

        Args:
            fetch: 'all', 'one', or None

        Returns:
            Query results
        """
        query, params = self.build()

        with self.db.get_connection() as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()

            cursor.execute(query, params)

            if fetch == 'one':
                row = cursor.fetchone()
                return dict(row) if row else None
            elif fetch == 'all':
                return [dict(row) for row in cursor.fetchall()]
            else:
                return cursor

    def _reset_after_execute(self) -> 'QueryBuilder':
        """Reset builder state after execute."""
        self._reset()
        return self


class VulnerabilityQueryBuilder(QueryBuilder):
    """
    Specialized query builder for vulnerability queries.

    Provides convenience methods for common vulnerability query patterns.
    """

    def __init__(self, db: ResultsDatabase):
        super().__init__(db)
        self._from_table("vulnerabilities v")

    def with_host_info(self) -> 'VulnerabilityQueryBuilder':
        """Join host information."""
        return self.join("hosts h", "v.host_id = h.id")

    def with_subsystem(self) -> 'VulnerabilityQueryBuilder':
        """Join subsystem information."""
        self.with_host_info()
        return self.join("subsystems s", "h.subsystem_id = s.id")

    def with_web_finding(self) -> 'VulnerabilityQueryBuilder':
        """Join web finding details."""
        return self.join("web_findings wf", "v.id = wf.vulnerability_id")

    def by_severity(self, *severities: str) -> 'VulnerabilityQueryBuilder':
        """Filter by severity level(s)."""
        return self.where_in("v.severity", severities)

    def by_type(self, *types: str) -> 'VulnerabilityQueryBuilder':
        """Filter by vulnerability type(s)."""
        return self.where_in("v.vulnerability_type", types)

    def by_host(self, host_ip: str) -> 'VulnerabilityQueryBuilder':
        """Filter by host IP."""
        return self.where("h.ip_address = ?", host_ip)

    def by_subsystem(self, subsystem_name: str) -> 'VulnerabilityQueryBuilder':
        """Filter by subsystem name."""
        return self.where("s.name = ?", subsystem_name)

    def critical_first(self) -> 'VulnerabilityQueryBuilder':
        """Order by severity (critical first)."""
        # Custom order for severity
        return self.order_by(
            "CASE v.severity " +
            "WHEN 'Critical' THEN 1 " +
            "WHEN 'High' THEN 2 " +
            "WHEN 'Medium' THEN 3 " +
            "WHEN 'Low' THEN 4 " +
            "WHEN 'Info' THEN 5 " +
            "ELSE 6 END"
        )

    def recent_first(self) -> 'VulnerabilityQueryBuilder':
        """Order by discovery date (recent first)."""
        return self.order_by("v.discovered_at", "DESC")

    def high_value_first(self) -> 'VulnerabilityQueryBuilder':
        """Order by CVSS score (highest first)."""
        return self.order_by("v.cvss_score", "DESC")


class PortScanQueryBuilder(QueryBuilder):
    """
    Specialized query builder for port scan queries.
    """

    def __init__(self, db: ResultsDatabase):
        super().__init__(db)
        self._from_table("port_scan_results psr")

    def with_host_info(self) -> 'PortScanQueryBuilder':
        """Join host information."""
        return self.join("hosts h", "psr.host_id = h.id")

    def with_subsystem(self) -> 'PortScanQueryBuilder':
        """Join subsystem information."""
        self.with_host_info()
        return self.join("subsystems s", "h.subsystem_id = s.id")

    def by_host(self, host_ip: str) -> 'PortScanQueryBuilder':
        """Filter by host IP."""
        return self.where("h.ip_address = ?", host_ip)

    def by_state(self, *states: str) -> 'PortScanQueryBuilder':
        """Filter by port state(s)."""
        return self.where_in("psr.state", states)

    def by_service(self, *services: str) -> 'PortScanQueryBuilder':
        """Filter by service name(s)."""
        return self.where_in("psr.service", services)

    def by_scan_tool(self, tool: str) -> 'PortScanQueryBuilder':
        """Filter by scan tool."""
        return self.where("psr.scan_tool = ?", tool)

    def open_ports_only(self) -> 'PortScanQueryBuilder':
        """Filter for open ports only."""
        return self.by_state("open")


# Predefined queries
class PredefinedQueries:
    """
    Collection of commonly used queries for pentest data analysis.
    """

    def __init__(self, db: ResultsDatabase):
        self.db = db

    def get_critical_vulnerabilities(self, limit: int = None) -> List[Dict]:
        """
        Get all Critical severity vulnerabilities.

        Args:
            limit: Optional limit on number of results

        Returns:
            List of vulnerability dictionaries
        """
        with self.db.get_connection() as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()

            query = """
                SELECT
                    v.id, v.title, v.severity, v.cvss_score,
                    v.cwe_id, v.cve_id, v.discovered_at,
                    h.ip_address, h.hostname,
                    s.name as subsystem_name
                FROM vulnerabilities v
                JOIN hosts h ON v.host_id = h.id
                LEFT JOIN subsystems s ON h.subsystem_id = s.id
                WHERE v.severity = 'Critical'
                ORDER BY v.cvss_score DESC
            """

            if limit:
                query += f" LIMIT {limit}"

            cursor.execute(query)
            return [dict(row) for row in cursor.fetchall()]

    def get_vulnerability_by_host(self) -> List[Dict]:
        """
        Get vulnerability count grouped by host.

        Returns:
            List of host vulnerability counts
        """
        with self.db.get_connection() as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()

            cursor.execute("""
                SELECT
                    h.ip_address,
                    s.name as subsystem_name,
                    COUNT(*) as total_vulns,
                    COUNT(CASE WHEN v.severity = 'Critical' THEN 1 END) as critical_count,
                    COUNT(CASE WHEN v.severity = 'High' THEN 1 END) as high_count,
                    COUNT(CASE WHEN v.severity = 'Medium' THEN 1 END) as medium_count
                FROM hosts h
                JOIN vulnerabilities v ON h.id = v.host_id
                LEFT JOIN subsystems s ON h.subsystem_id = s.id
                GROUP BY h.id
                ORDER BY total_vulns DESC
            """)

            return [dict(row) for row in cursor.fetchall()]

    def get_exposed_services(self) -> List[Dict]:
        """
        Get all exposed services across all hosts.

        Returns:
            List of exposed services
        """
        with self.db.get_connection() as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()

            cursor.execute("""
                SELECT
                    psr.service,
                    psr.version,
                    COUNT(*) as host_count,
                    GROUP_CONCAT(DISTINCT h.ip_address) as hosts
                FROM port_scan_results psr
                JOIN hosts h ON psr.host_id = h.id
                WHERE psr.state = 'open'
                AND psr.service IS NOT NULL
                GROUP BY psr.service, psr.version
                ORDER BY host_count DESC
            """)

            return [dict(row) for row in cursor.fetchall()]

    def get_most_vulnerable_subsystems(self, limit: int = 10) -> List[Dict]:
        """
        Get subsystems ranked by vulnerability count.

        Args:
            limit: Maximum number of results

        Returns:
            List of subsystems with vulnerability counts
        """
        with self.db.get_connection() as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()

            query = """
                SELECT
                    s.name as subsystem_name,
                    s.description,
                    COUNT(DISTINCT h.id) as host_count,
                    COUNT(v.id) as total_vulns,
                    COUNT(CASE WHEN v.severity = 'Critical' THEN 1 END) as critical_count,
                    COUNT(CASE WHEN v.severity = 'High' THEN 1 END) as high_count,
                    AVG(v.cvss_score) as avg_cvss
                FROM subsystems s
                JOIN hosts h ON s.id = h.subsystem_id
                JOIN vulnerabilities v ON h.id = v.host_id
                GROUP BY s.id
                ORDER BY total_vulns DESC
            """

            if limit:
                query += f" LIMIT {limit}"

            cursor.execute(query)
            return [dict(row) for row in cursor.fetchall()]

    def get_vulnerability_type_trend(self) -> List[Dict]:
        """
        Get vulnerability discovery trend by type.

        Returns:
            List of vulnerability types with discovery counts
        """
        with self.db.get_connection() as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()

            cursor.execute("""
                SELECT
                    vulnerability_type,
                    DATE(discovered_at) as discovery_date,
                    COUNT(*) as count
                FROM vulnerabilities
                GROUP BY vulnerability_type, DATE(discovered_at)
                ORDER BY discovery_date DESC, count DESC
            """)

            return [dict(row) for row in cursor.fetchall()]

    def get_scan_coverage(self) -> List[Dict]:
        """
        Get scan coverage statistics by subsystem.

        Returns:
            List of subsystems with scan statistics
        """
        with self.db.get_connection() as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()

            cursor.execute("""
                SELECT
                    s.name as subsystem_name,
                    COUNT(DISTINCT h.id) as total_hosts,
                    COUNT(DISTINCT CASE WHEN psr.id IS NOT NULL THEN h.id END) as scanned_hosts,
                    COUNT(psr.id) as total_ports,
                    COUNT(CASE WHEN psr.state = 'open' THEN 1 END) as open_ports
                FROM subsystems s
                JOIN hosts h ON s.id = h.subsystem_id
                LEFT JOIN port_scan_results psr ON h.id = psr.host_id
                GROUP BY s.id
                ORDER BY scanned_hosts DESC
            """)

            return [dict(row) for row in cursor.fetchall()]


def main():
    """CLI interface for query builder testing."""
    import argparse

    parser = argparse.ArgumentParser(description="Query builder CLI")
    parser.add_argument("--db-path", default="./data/results.db", help="Database path")
    parser.add_argument("--query", choices=[
        "critical", "by_host", "services", "subsystems", "trend", "coverage"
    ], help="Predefined query to run")
    parser.add_argument("--limit", type=int, help="Limit results")

    args = parser.parse_args()

    db = ResultsDatabase(args.db_path)
    queries = PredefinedQueries(db)

    if args.query == "critical":
        results = queries.get_critical_vulnerabilities(args.limit)
        print(f"\n=== Critical Vulnerabilities ===")
        for r in results:
            print(f"[{r['id']}] {r['title']} ({r['ip_address']}) - CVSS: {r['cvss_score']}")

    elif args.query == "by_host":
        results = queries.get_vulnerability_by_host()
        print(f"\n=== Vulnerabilities by Host ===")
        for r in results:
            print(f"{r['ip_address']}: {r['total_vulns']} vulns (C:{r['critical_count']} H:{r['high_count']} M:{r['medium_count']})")

    elif args.query == "services":
        results = queries.get_exposed_services()
        print(f"\n=== Exposed Services ===")
        for r in results:
            print(f"{r['service']} {r['version']}: {r['host_count']} hosts")

    elif args.query == "subsystems":
        results = queries.get_most_vulnerable_subsystems(args.limit)
        print(f"\n=== Most Vulnerable Subsystems ===")
        for r in results:
            print(f"{r['subsystem_name']}: {r['total_vulns']} vulns, {r['host_count']} hosts, {r['critical_count']} critical")

    elif args.query == "trend":
        results = queries.get_vulnerability_type_trend()
        print(f"\n=== Vulnerability Discovery Trend ===")
        for r in results[:20]:
            print(f"{r['discovery_date']}: {r['vulnerability_type']} - {r['count']} discovered")

    elif args.query == "coverage":
        results = queries.get_scan_coverage()
        print(f"\n=== Scan Coverage by Subsystem ===")
        for r in results:
            print(f"{r['subsystem_name']}: {r['scanned_hosts']}/{r['total_hosts']} hosts, {r['open_ports']}/{r['total_ports']} open ports")

    db.close()


if __name__ == "__main__":
    main()
