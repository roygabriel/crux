import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import styles from './index.module.css';

const features = [
  {
    title: 'Multi-Agent Orchestration',
    icon: '🎯',
    description:
      'Coordinate Claude Code, Codex CLI, Gemini CLI, or any CLI tool across tmux sessions. Assign tasks, monitor progress, and resolve conflicts automatically.',
  },
  {
    title: 'Persistent Memory',
    icon: '🧠',
    description:
      'Three-layer memory system — markdown memory bank, SQLite with FTS5, and vector search via chromem-go. Every decision is recorded and searchable, even across sessions.',
  },
  {
    title: 'Phase-Based Workflows',
    icon: '📋',
    description:
      'Two-document phase system with specs and prompt contracts. Verification gates enforce quality — the engine never advances until go build, go test, and go vet pass.',
  },
  {
    title: 'Plugin Architecture',
    icon: '🔌',
    description:
      'Agents are plugins, not hardcoded. Add support for any CLI tool by implementing one Go interface or configuring regex patterns in YAML.',
  },
  {
    title: 'Security First',
    icon: '🔒',
    description:
      'Four-tier permission model, filesystem sandboxing, structured audit logging, and per-agent rate limiting. All agent commits land on feature branches.',
  },
  {
    title: 'Zero Infrastructure',
    icon: '📦',
    description:
      'Single binary, go build produces one executable. SQLite for structured data, chromem-go for vectors — both embedded. No Postgres, no Redis, no Docker.',
  },
];

function Feature({ title, icon, description }) {
  return (
    <div className={styles.featureCard}>
      <div className={styles.featureIcon}>{icon}</div>
      <Heading as="h3" className={styles.featureTitle}>{title}</Heading>
      <p className={styles.featureDescription}>{description}</p>
    </div>
  );
}

function HeroSection() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <header className={styles.hero}>
      <div className={styles.heroInner}>
        <div className={styles.heroContent}>
          <div className={styles.heroBadge}>
            <span>🚧 Under Active Development</span>
          </div>
          <Heading as="h1" className={styles.heroTitle}>
            Orchestrate AI Coding Agents
            <span className={styles.heroTitleGradient}> with Persistent Memory</span>
          </Heading>
          <p className={styles.heroSubtitle}>
            Crux is a single-binary Go tool that coordinates multiple AI coding agents across tmux sessions.
            Phase-based workflows, verification gates, and vector-searchable decision tracking — so your
            orchestrator never loses track of what agents are doing.
          </p>
          <div className={styles.heroActions}>
            <Link className={styles.heroPrimary} to="/docs/getting-started/installation">
              Get Started →
            </Link>
            <Link className={styles.heroSecondary} href="https://github.com/roygabriel/crux">
              View on GitHub
            </Link>
          </div>
        </div>
        <div className={styles.heroTerminal}>
          <div className={styles.terminalWindow}>
            <div className={styles.terminalDots}>
              <span className={styles.dotRed}></span>
              <span className={styles.dotYellow}></span>
              <span className={styles.dotGreen}></span>
            </div>
            <pre className={styles.terminalCode}>
              <code>
                {`$ go install github.com/roygabriel/crux/cmd/crux@latest

$ crux init --example httpapi
✓ Created .crux/config.yaml
✓ Created docs/phases/PHASE1A.md
✓ Created docs/phases/PHASE1A-PROMPT.md

$ crux start
▶ Orchestrator loop started
  ├─ claude-1: Phase 1A, Prompt 1/3 (busy)
  ├─ codex-1:  Phase 1B, Prompt 1/2 (busy)
  └─ gemini-1: Phase 1A, Prompt 2/3 (idle)

$ crux decisions search "chose chi over gorilla"
┌─ Decision dec-a3f2
│  Phase: 2A · Prompt: 2
│  Context: needed HTTP router
│  Action: selected chi router
│  Rationale: better middleware composition
└─ Outcome: implemented successfully`}
              </code>
            </pre>
          </div>
        </div>
      </div>
    </header>
  );
}

function FeaturesSection() {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className={styles.featuresHeader}>
          <Heading as="h2" className={styles.sectionTitle}>
            Everything You Need to Orchestrate AI Agents
          </Heading>
          <p className={styles.sectionSubtitle}>
            Built for developers who want reliable, repeatable multi-agent workflows without the infrastructure overhead.
          </p>
        </div>
        <div className={styles.featuresGrid}>
          {features.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}

function ArchitectureSection() {
  const layers = [
    {
      label: 'Orchestrator Loop',
      color: '#D97706',
      components: ['World State', 'Agent Assignment', 'Decision RAG', 'Gates'],
    },
    {
      label: 'Core Engines',
      color: '#B45309',
      components: ['Phase Engine', 'Memory System', 'Agent Manager', 'Security Layer'],
    },
    {
      label: 'Plugin Layer',
      color: '#F59E0B',
      components: ['Claude Code', 'Codex CLI', 'Gemini CLI', 'Generic Adapter'],
    },
    {
      label: 'Transport',
      color: '#0D9488',
      components: ['tmux Sessions', 'Panes', 'capture-pane', 'send-keys'],
    },
  ];

  return (
    <section className={styles.architecture}>
      <div className="container">
        <div className={styles.featuresHeader}>
          <Heading as="h2" className={styles.sectionTitle}>
            Six-Layer Architecture
          </Heading>
          <p className={styles.sectionSubtitle}>
            Clean separation of concerns — from the orchestrator loop down to tmux transport.
          </p>
        </div>
        <div className={styles.archLayers}>
          {layers.map((layer, idx) => (
            <div key={idx} className={styles.archLayer} style={{ '--layer-color': layer.color }}>
              <div className={styles.archLayerLabel}>{layer.label}</div>
              <div className={styles.archLayerComponents}>
                {layer.components.map((comp, i) => (
                  <span key={i} className={styles.archChip}>{comp}</span>
                ))}
              </div>
            </div>
          ))}
        </div>
        <div className={styles.archCTA}>
          <Link className={styles.heroPrimary} to="/docs/concepts/architecture">
            Explore Architecture →
          </Link>
        </div>
      </div>
    </section>
  );
}

function CTASection() {
  return (
    <section className={styles.cta}>
      <div className="container">
        <div className={styles.ctaInner}>
          <Heading as="h2" className={styles.ctaTitle}>
            Ready to orchestrate?
          </Heading>
          <p className={styles.ctaSubtitle}>
            Get up and running in under 5 minutes. Single binary, zero infrastructure.
          </p>
          <div className={styles.heroActions}>
            <Link className={styles.heroPrimary} to="/docs/getting-started/installation">
              Install Crux →
            </Link>
            <Link className={styles.heroSecondary} to="/docs/getting-started/first-project">
              First Project Tutorial
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}

export default function Home() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <Layout
      title="Orchestrate AI Coding Agents"
      description="A single-binary Go orchestrator that coordinates AI coding agents across tmux sessions with persistent memory, phase-based workflows, and vector-searchable decision tracking.">
      <HeroSection />
      <main>
        <FeaturesSection />
        <ArchitectureSection />
        <CTASection />
      </main>
    </Layout>
  );
}
