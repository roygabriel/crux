import React, { useState, useMemo } from 'react';
import Layout from '@theme/Layout';
import roadmapData from '../data/roadmap.json';
import styles from './roadmap.module.css';

/** Lightweight markdown → HTML for issue body descriptions. */
function simpleMarkdown(text) {
    if (!text) return '';
    return text
        // Escape HTML
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        // Headings (## → h4, ### → h5, keep them small in modal)
        .replace(/^### (.+)$/gm, '<h5>$1</h5>')
        .replace(/^## (.+)$/gm, '<h4>$1</h4>')
        .replace(/^# (.+)$/gm, '<h4>$1</h4>')
        // Bold & italic
        .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
        .replace(/\*(.+?)\*/g, '<em>$1</em>')
        // Inline code
        .replace(/`([^`]+)`/g, '<code>$1</code>')
        // Unordered list items
        .replace(/^- (.+)$/gm, '<li>$1</li>')
        // Wrap consecutive <li> in <ul>
        .replace(/((?:<li>.*<\/li>\n?)+)/g, '<ul>$1</ul>')
        // Paragraphs (double newlines)
        .replace(/\n\n/g, '</p><p>')
        // Single newlines → <br>
        .replace(/\n/g, '<br/>')
        // Wrap in paragraph
        .replace(/^(.+)$/, '<p>$1</p>');
}

const STATUS_CONFIG = {
    Done: { className: 'statusDone', icon: '✓' },
    'In Progress': { className: 'statusInProgress', icon: '▶' },
    Todo: { className: 'statusTodo', icon: '○' },
};

function StatusBadge({ status }) {
    const config = STATUS_CONFIG[status] || STATUS_CONFIG.Todo;
    return (
        <span className={`${styles.statusBadge} ${styles[config.className]}`}>
            <span className={styles.statusIcon}>{config.icon}</span>
            {status}
        </span>
    );
}

function LabelBadge({ label }) {
    return <span className={styles.labelBadge}>{label}</span>;
}

function ItemModal({ item, onClose }) {
    if (!item) return null;
    return (
        <div className={styles.modalOverlay} onClick={onClose}>
            <div className={styles.modalContent} onClick={(e) => e.stopPropagation()}>
                <button className={styles.modalClose} onClick={onClose}>
                    ✕
                </button>
                <h2 className={styles.modalTitle}>{item.title}</h2>
                <div className={styles.modalMeta}>
                    <StatusBadge status={item.status} />
                    {item.labels?.map((label) => (
                        <LabelBadge key={label} label={label} />
                    ))}
                </div>
                {item.description && (
                    <div
                        className={styles.modalDescription}
                        dangerouslySetInnerHTML={{ __html: simpleMarkdown(item.description) }}
                    />
                )}
                {item.url && (
                    <a
                        href={item.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className={styles.modalLink}
                    >
                        View on GitHub →
                    </a>
                )}
            </div>
        </div>
    );
}

function RoadmapItem({ item, onClick }) {
    const config = STATUS_CONFIG[item.status] || STATUS_CONFIG.Todo;
    return (
        <button
            className={`${styles.itemCard} ${styles[config.className + 'Card']}`}
            onClick={() => onClick(item)}
            type="button"
        >
            <div className={styles.itemHeader}>
                <StatusBadge status={item.status} />
            </div>
            <h3 className={styles.itemTitle}>{item.title}</h3>
            {item.labels?.length > 0 && (
                <div className={styles.itemLabels}>
                    {item.labels.map((label) => (
                        <LabelBadge key={label} label={label} />
                    ))}
                </div>
            )}
        </button>
    );
}

function QuarterColumn({ quarter, onItemClick }) {
    const doneCount = quarter.items.filter((i) => i.status === 'Done').length;
    const total = quarter.items.length;
    const progress = total > 0 ? Math.round((doneCount / total) * 100) : 0;

    return (
        <div className={styles.quarterColumn}>
            <div className={styles.quarterHeader}>
                <h2 className={styles.quarterLabel}>{quarter.label}</h2>
                <div className={styles.quarterProgress}>
                    <div className={styles.progressBar}>
                        <div
                            className={styles.progressFill}
                            style={{ width: `${progress}%` }}
                        />
                    </div>
                    <span className={styles.progressText}>
                        {doneCount}/{total}
                    </span>
                </div>
            </div>
            <div className={styles.quarterItems}>
                {quarter.items.map((item, idx) => (
                    <RoadmapItem key={idx} item={item} onClick={onItemClick} />
                ))}
            </div>
        </div>
    );
}

export default function Roadmap() {
    const [selectedItem, setSelectedItem] = useState(null);

    const lastUpdated = new Date(roadmapData.lastUpdated).toLocaleDateString(
        'en-US',
        {
            year: 'numeric',
            month: 'long',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit',
        }
    );

    return (
        <Layout
            title="Roadmap"
            description="See what's planned, in progress, and shipped for Crux."
        >
            <main className={styles.roadmapPage}>
                <div className={styles.heroSection}>
                    <h1 className={styles.pageTitle}>
                        <span className={styles.titleAccent}>Product</span> Roadmap
                    </h1>
                    <p className={styles.pageSubtitle}>
                        Follow our journey from early alpha to production-ready orchestrator.
                        Click any item for details.
                    </p>
                    <div className={styles.legend}>
                        <span className={`${styles.legendItem} ${styles.statusDone}`}>
                            ✓ Done
                        </span>
                        <span
                            className={`${styles.legendItem} ${styles.statusInProgress}`}
                        >
                            ▶ In Progress
                        </span>
                        <span className={`${styles.legendItem} ${styles.statusTodo}`}>
                            ○ Planned
                        </span>
                    </div>
                </div>

                {/* Timeline connector */}
                <div className={styles.timelineContainer}>
                    <div className={styles.timelineLine} />
                    <div className={styles.timelineQuarters}>
                        {roadmapData.quarters.map((quarter, idx) => (
                            <QuarterColumn
                                key={idx}
                                quarter={quarter}
                                onItemClick={setSelectedItem}
                            />
                        ))}
                    </div>
                </div>

                <footer className={styles.roadmapFooter}>
                    <p>
                        Last synced: {lastUpdated} ·{' '}
                        <a
                            href="https://github.com/users/roygabriel/projects/2"
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            View full board on GitHub
                        </a>
                    </p>
                </footer>
            </main>

            <ItemModal item={selectedItem} onClose={() => setSelectedItem(null)} />
        </Layout>
    );
}
