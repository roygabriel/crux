#!/usr/bin/env node
/**
 * fetch-roadmap.mjs
 *
 * Fetches items from GitHub Projects v2 board and writes roadmap.json
 * for the Docusaurus site. Requires a PAT with read:project scope.
 *
 * Usage:
 *   PROJECT_PAT=ghp_xxx node scripts/fetch-roadmap.mjs
 *
 * Environment variables:
 *   PROJECT_PAT   - GitHub Personal Access Token with read:project scope
 *   PROJECT_OWNER - GitHub username (default: roygabriel)
 *   PROJECT_NUM   - Project number (default: 2)
 */

import { writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

const TOKEN = process.env.PROJECT_PAT;
const OWNER = process.env.PROJECT_OWNER || 'roygabriel';
const PROJECT_NUM = parseInt(process.env.PROJECT_NUM || '2', 10);
const OUTPUT = join(__dirname, '..', 'site', 'src', 'data', 'roadmap.json');

if (!TOKEN) {
    console.error('ERROR: PROJECT_PAT environment variable is required.');
    process.exit(1);
}

const QUERY = `
query($owner: String!, $number: Int!) {
  user(login: $owner) {
    projectV2(number: $number) {
      title
      items(first: 100, orderBy: {field: POSITION, direction: ASC}) {
        nodes {
          fieldValues(first: 20) {
            nodes {
              ... on ProjectV2ItemFieldTextValue {
                text
                field { ... on ProjectV2Field { name } }
              }
              ... on ProjectV2ItemFieldSingleSelectValue {
                name
                field { ... on ProjectV2SingleSelectField { name } }
              }
              ... on ProjectV2ItemFieldDateValue {
                date
                field { ... on ProjectV2Field { name } }
              }
              ... on ProjectV2ItemFieldIterationValue {
                title
                startDate
                duration
                field { ... on ProjectV2IterationField { name } }
              }
            }
          }
          content {
            ... on Issue {
              title
              body
              number
              url
              state
              labels(first: 10) {
                nodes { name color }
              }
            }
            ... on DraftIssue {
              title
              body
            }
            ... on PullRequest {
              title
              body
              number
              url
              state
            }
          }
        }
      }
    }
  }
}
`;

async function fetchProject() {
    const res = await fetch('https://api.github.com/graphql', {
        method: 'POST',
        headers: {
            Authorization: `Bearer ${TOKEN}`,
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            query: QUERY,
            variables: { owner: OWNER, number: PROJECT_NUM },
        }),
    });

    if (!res.ok) {
        throw new Error(`GitHub API returned ${res.status}: ${await res.text()}`);
    }

    const json = await res.json();

    if (json.errors) {
        throw new Error(`GraphQL errors: ${JSON.stringify(json.errors, null, 2)}`);
    }

    return json.data.user.projectV2;
}

function extractFieldValue(fieldNodes, fieldName) {
    for (const node of fieldNodes) {
        const name = node.field?.name;
        if (name?.toLowerCase() === fieldName.toLowerCase()) {
            return node.name || node.text || node.title || node.date || null;
        }
    }
    return null;
}

function mapStatus(raw) {
    if (!raw) return 'Todo';
    const lower = raw.toLowerCase();
    if (lower.includes('done') || lower.includes('complete')) return 'Done';
    if (lower.includes('progress') || lower.includes('active') || lower.includes('current')) return 'In Progress';
    return 'Todo';
}

function inferQuarter(fieldNodes) {
    // Try explicit "Quarter" field first
    const quarter = extractFieldValue(fieldNodes, 'Quarter');
    if (quarter) return quarter;

    // Try "Target" or "Milestone" fields
    const target = extractFieldValue(fieldNodes, 'Target')
        || extractFieldValue(fieldNodes, 'Milestone');
    if (target && /Q[1-4]\s*\d{4}/i.test(target)) return target;

    // Try iteration field → map to quarter
    for (const node of fieldNodes) {
        if (node.startDate) {
            const date = new Date(node.startDate);
            const q = Math.ceil((date.getMonth() + 1) / 3);
            return `Q${q} ${date.getFullYear()}`;
        }
    }

    // Fallback: "Unscheduled"
    return 'Unscheduled';
}

function processItems(project) {
    const quarterMap = new Map();

    for (const item of project.items.nodes) {
        const content = item.content;
        if (!content || !content.title) continue;

        const fieldNodes = item.fieldValues?.nodes || [];

        const status = mapStatus(extractFieldValue(fieldNodes, 'Status'));
        const quarter = inferQuarter(fieldNodes);
        const labels = content.labels?.nodes?.map((l) => l.name) || [];
        const description = (content.body || '').slice(0, 500);

        const roadmapItem = {
            title: content.title,
            status,
            description,
            labels,
            url: content.url || '',
        };

        if (!quarterMap.has(quarter)) {
            quarterMap.set(quarter, []);
        }
        quarterMap.get(quarter).push(roadmapItem);
    }

    // Sort quarters chronologically
    const sortedQuarters = [...quarterMap.entries()].sort((a, b) => {
        if (a[0] === 'Unscheduled') return 1;
        if (b[0] === 'Unscheduled') return -1;
        return a[0].localeCompare(b[0]);
    });

    return sortedQuarters.map(([label, items]) => ({ label, items }));
}

async function main() {
    console.log(`Fetching project #${PROJECT_NUM} for ${OWNER}...`);

    const project = await fetchProject();
    console.log(`Project: ${project.title}`);
    console.log(`Items: ${project.items.nodes.length}`);

    const quarters = processItems(project);
    const totalItems = quarters.reduce((sum, q) => sum + q.items.length, 0);
    console.log(`Grouped into ${quarters.length} quarters, ${totalItems} items`);

    const roadmap = {
        lastUpdated: new Date().toISOString(),
        quarters,
    };

    writeFileSync(OUTPUT, JSON.stringify(roadmap, null, 2) + '\n');
    console.log(`Written to ${OUTPUT}`);
}

main().catch((err) => {
    console.error('Failed to fetch roadmap:', err.message);
    process.exit(1);
});
