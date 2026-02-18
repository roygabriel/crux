import React from 'react';
import Layout from '@theme/Layout';
import styles from './styles.module.css';

export default function NotFound() {
    return (
        <Layout title="Page Not Found" description="The page you're looking for doesn't exist.">
            <main className={styles.container}>
                <div className={styles.content}>
                    <img
                        src="/img/404.png"
                        alt="404 - Page Not Found"
                        className={styles.image}
                    />
                    <h1 className={styles.heading}>Lost in the phase graph</h1>
                    <p className={styles.message}>
                        This page doesn't exist, was moved, or the agent working on it
                        hasn't finished yet.
                    </p>
                    <div className={styles.actions}>
                        <a href="/" className={styles.primaryBtn}>
                            Back to Home
                        </a>
                        <a href="/docs/getting-started/installation" className={styles.secondaryBtn}>
                            Read the Docs
                        </a>
                    </div>
                    <div className={styles.terminal}>
                        <div className={styles.terminalBar}>
                            <span className={styles.dot} style={{ background: '#ef4444' }} />
                            <span className={styles.dot} style={{ background: '#f59e0b' }} />
                            <span className={styles.dot} style={{ background: '#22c55e' }} />
                        </div>
                        <pre className={styles.terminalBody}>
                            <code>
                                {`$ crux phase show 404
Error: phase "404" not found

Hint: run 'crux phase list' to see available phases`}
                            </code>
                        </pre>
                    </div>
                </div>
            </main>
        </Layout>
    );
}
