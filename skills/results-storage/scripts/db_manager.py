#!/usr/bin/env python3
"""
Database Manager for Pentest Results Storage

Handles SQLite database initialization, table creation, and connection management.
Provides the ResultsDatabase class for low-level database operations.
"""

import sqlite3
import os
from pathlib import Path
from typing import Optional
from datetime import datetime
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class ResultsDatabase:
    """
    SQLite database manager for pentest results storage.

    Handles database initialization, schema creation, and provides
    low-level database connection management.
    """

    # Database schema version
    SCHEMA_VERSION = "1.0.0"

    # SQL statements for table creation
    TABLES_SQL = {
        "subsystems": """
            CREATE TABLE IF NOT EXISTS subsystems (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                name TEXT NOT NULL UNIQUE,
                description TEXT,
                subnet_range TEXT,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
            )
        """,

        "hosts": """
            CREATE TABLE IF NOT EXISTS hosts (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                subsystem_id INTEGER,
                ip_address TEXT NOT NULL,
                hostname TEXT,
                mac_address TEXT,
                os_fingerprint TEXT,
                status TEXT DEFAULT 'unknown',
                first_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                last_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                UNIQUE(subsystem_id, ip_address),
                FOREIGN KEY (subsystem_id) REFERENCES subsystems(id) ON DELETE CASCADE
            )
        """,

        "vulnerabilities": """
            CREATE TABLE IF NOT EXISTS vulnerabilities (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                host_id INTEGER NOT NULL,
                vulnerability_type TEXT NOT NULL,
                severity TEXT NOT NULL,
                title TEXT NOT NULL,
                description TEXT,
                affected_component TEXT,
                proof_of_concept TEXT,
                cvss_score REAL,
                cwe_id TEXT,
                cve_id TEXT,
                status TEXT DEFAULT 'open',
                discovered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                discovered_by_skill TEXT,
                FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
            )
        """,

        "port_scan_results": """
            CREATE TABLE IF NOT EXISTS port_scan_results (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                host_id INTEGER NOT NULL,
                port INTEGER NOT NULL,
                protocol TEXT NOT NULL,
                state TEXT NOT NULL,
                service TEXT,
                version TEXT,
                product TEXT,
                extra_info TEXT,
                scan_tool TEXT,
                scan_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE,
                UNIQUE(host_id, port, protocol)
            )
        """,

        "web_findings": """
            CREATE TABLE IF NOT EXISTS web_findings (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                vulnerability_id INTEGER NOT NULL,
                url TEXT NOT NULL,
                parameter TEXT,
                http_method TEXT,
                payload TEXT,
                response_evidence TEXT,
                context TEXT,
                request_headers TEXT,
                FOREIGN KEY (vulnerability_id) REFERENCES vulnerabilities(id) ON DELETE CASCADE
            )
        """,

        "scan_metadata": """
            CREATE TABLE IF NOT EXISTS scan_metadata (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                host_id INTEGER NOT NULL,
                skill_used TEXT NOT NULL,
                scan_command TEXT,
                scan_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                raw_output_path TEXT,
                notes TEXT,
                FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
            )
        """
    }

    # Index creation SQL
    INDEXES_SQL = [
        "CREATE INDEX IF NOT EXISTS idx_vulnerabilities_severity ON vulnerabilities(severity)",
        "CREATE INDEX IF NOT EXISTS idx_vulnerabilities_type ON vulnerabilities(vulnerability_type)",
        "CREATE INDEX IF NOT EXISTS idx_vulnerabilities_host ON vulnerabilities(host_id)",
        "CREATE INDEX IF NOT EXISTS idx_vulnerabilities_discovered_at ON vulnerabilities(discovered_at)",
        "CREATE INDEX IF NOT EXISTS idx_vulnerabilities_status ON vulnerabilities(status)",
        "CREATE INDEX IF NOT EXISTS idx_host_ips ON hosts(ip_address)",
        "CREATE INDEX IF NOT EXISTS idx_host_subsystem ON hosts(subsystem_id)",
        "CREATE INDEX IF NOT EXISTS idx_port_scans_host ON port_scan_results(host_id)",
        "CREATE INDEX IF NOT EXISTS idx_port_scans_state ON port_scan_results(state)",
        "CREATE INDEX IF NOT EXISTS idx_web_findings_vuln ON web_findings(vulnerability_id)",
        "CREATE INDEX IF NOT EXISTS idx_scan_metadata_host ON scan_metadata(host_id)",
        "CREATE INDEX IF NOT EXISTS idx_scan_metadata_skill ON scan_metadata(skill_used)"
    ]

    def __init__(self, db_path: str = "./data/results.db"):
        """
        Initialize the database manager.

        Args:
            db_path: Path to SQLite database file (default: ./data/results.db)
        """
        self.db_path = os.path.abspath(db_path)
        self._connection = None

        # Ensure data directory exists
        self._ensure_data_directory()

        # Initialize database
        self._initialize_database()

        logger.info(f"Database initialized at: {self.db_path}")

    def _ensure_data_directory(self):
        """Create the data directory if it doesn't exist."""
        data_dir = os.path.dirname(self.db_path)

        if data_dir and not os.path.exists(data_dir):
            os.makedirs(data_dir, exist_ok=True)
            logger.info(f"Created data directory: {data_dir}")

        # Set restrictive permissions on data directory
        if data_dir:
            os.chmod(data_dir, 0o700)

    def _initialize_database(self):
        """
        Initialize database schema.

        Creates all tables and indexes if they don't exist.
        Sets appropriate file permissions.
        """
        # Check if database exists
        is_new_db = not os.path.exists(self.db_path)

        with self.get_connection() as conn:
            cursor = conn.cursor()

            # Create all tables
            for table_name, table_sql in self.TABLES_SQL.items():
                try:
                    cursor.execute(table_sql)
                    logger.debug(f"Table {table_name} ready")
                except sqlite3.Error as e:
                    logger.error(f"Failed to create table {table_name}: {e}")
                    raise

            # Create all indexes
            for index_sql in self.INDEXES_SQL:
                try:
                    cursor.execute(index_sql)
                except sqlite3.Error as e:
                    logger.warning(f"Failed to create index: {e}")

            # Create metadata table for schema version
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS metadata (
                    key TEXT PRIMARY KEY,
                    value TEXT,
                    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )
            """)

            # Store schema version
            cursor.execute("""
                INSERT OR REPLACE INTO metadata (key, value, updated_at)
                VALUES ('schema_version', ?, CURRENT_TIMESTAMP)
            """, (self.SCHEMA_VERSION,))

            conn.commit()

        # Set file permissions (owner read/write only)
        if is_new_db:
            os.chmod(self.db_path, 0o600)
            logger.info(f"Created new database with schema version {self.SCHEMA_VERSION}")
        else:
            logger.debug(f"Using existing database at {self.db_path}")

    def get_connection(self) -> sqlite3.Connection:
        """
        Get a database connection.

        Returns:
            SQLite connection object

        Note:
            Connection should be used as a context manager:
            with db.get_connection() as conn:
                # use conn
        """
        if self._connection is None:
            self._connection = sqlite3.connect(self.db_path)
            # Enable foreign keys
            self._connection.execute("PRAGMA foreign_keys = ON")
            # Set WAL mode for better concurrency
            self._connection.execute("PRAGMA journal_mode = WAL")
            # Set busy timeout (5 seconds)
            self._connection.execute("PRAGMA busy_timeout = 5000")

        return self._connection

    def close(self):
        """Close the database connection."""
        if self._connection:
            self._connection.close()
            self._connection = None
            logger.info("Database connection closed")

    def __enter__(self):
        """Context manager entry."""
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit."""
        self.close()

    def execute_query(self, query: str, params: tuple = None, fetch: str = None):
        """
        Execute a SQL query.

        Args:
            query: SQL query string
            params: Query parameters (optional)
            fetch: Fetch type - 'one', 'all', or None (default)

        Returns:
            Fetched results if fetch specified, else cursor

        Raises:
            sqlite3.Error: On database errors
        """
        with self.get_connection() as conn:
            cursor = conn.cursor()

            try:
                if params:
                    cursor.execute(query, params)
                else:
                    cursor.execute(query)

                if fetch == 'one':
                    return cursor.fetchone()
                elif fetch == 'all':
                    return cursor.fetchall()
                else:
                    return cursor

            except sqlite3.Error as e:
                logger.error(f"Query failed: {query[:100]}... Error: {e}")
                raise

    def execute_many(self, query: str, params_list: list):
        """
        Execute a SQL query multiple times with different parameters.

        Args:
            query: SQL query string
            params_list: List of parameter tuples

        Returns:
            Row count
        """
        with self.get_connection() as conn:
            cursor = conn.cursor()

            try:
                cursor.executemany(query, params_list)
                conn.commit()
                return cursor.rowcount
            except sqlite3.Error as e:
                logger.error(f"Batch execute failed: {e}")
                raise

    def get_schema_version(self) -> str:
        """
        Get the current database schema version.

        Returns:
            Schema version string
        """
        result = self.execute_query(
            "SELECT value FROM metadata WHERE key = 'schema_version'",
            fetch='one'
        )

        return result[0] if result else "unknown"

    def vacuum(self):
        """Vacuum the database to reclaim space."""
        logger.info("Vacuuming database...")
        with self.get_connection() as conn:
            conn.execute("VACUUM")
            conn.commit()
        logger.info("Database vacuum complete")

    def analyze(self):
        """Analyze database tables for query optimization."""
        logger.info("Analyzing database tables...")
        with self.get_connection() as conn:
            conn.execute("ANALYZE")
            conn.commit()
        logger.info("Database analysis complete")

    def get_database_info(self) -> dict:
        """
        Get database information and statistics.

        Returns:
            Dict with database stats
        """
        info = {
            "path": self.db_path,
            "schema_version": self.get_schema_version(),
            "size_bytes": os.path.getsize(self.db_path) if os.path.exists(self.db_path) else 0,
            "tables": {},
            "indexes": []
        }

        with self.get_connection() as conn:
            cursor = conn.cursor()

            # Get table row counts
            for table in self.TABLES_SQL.keys():
                cursor.execute(f"SELECT COUNT(*) FROM {table}")
                info["tables"][table] = cursor.fetchone()[0]

            # Get index information
            cursor.execute("""
                SELECT name, tbl_name
                FROM sqlite_master
                WHERE type = 'index'
                AND name NOT LIKE 'sqlite_%'
                ORDER BY tbl_name, name
            """)

            info["indexes"] = [
                {"name": row[0], "table": row[1]}
                for row in cursor.fetchall()
            ]

        return info


def main():
    """CLI interface for database management."""
    import argparse
    import json

    parser = argparse.ArgumentParser(
        description="Manage pentest results database"
    )
    parser.add_argument(
        "--db-path",
        default="./data/results.db",
        help="Path to database file"
    )
    parser.add_argument(
        "--action",
        choices=["info", "vacuum", "analyze", "reset"],
        default="info",
        help="Action to perform"
    )

    args = parser.parse_args()

    if args.action == "reset":
        # Dangerous operation - require confirmation
        response = input("This will DELETE ALL DATA. Type 'DELETE' to confirm: ")
        if response == "DELETE":
            if os.path.exists(args.db_path):
                os.remove(args.db_path)
                print(f"Database deleted: {args.db_path}")
            else:
                print(f"Database not found: {args.db_path}")
        else:
            print("Reset cancelled")
        return

    # All other actions require database instance
    db = ResultsDatabase(args.db_path)

    if args.action == "info":
        info = db.get_database_info()
        print("\n=== Database Information ===")
        print(f"Path: {info['path']}")
        print(f"Schema Version: {info['schema_version']}")
        print(f"Size: {info['size_bytes']:,} bytes ({info['size_bytes'] / 1024:.2f} KB)")
        print("\nTables:")
        for table, count in info['tables'].items():
            print(f"  {table}: {count:,} rows")
        print(f"\nIndexes: {len(info['indexes'])}")

    elif args.action == "vacuum":
        db.vacuum()

    elif args.action == "analyze":
        db.analyze()

    db.close()


if __name__ == "__main__":
    main()
