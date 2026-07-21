const activeFlagsState = new Map();
const auditTrailState = [];
let renderTimeout = null;

function scheduleRenderFlags() {
    if (renderTimeout) return;
    renderTimeout = setTimeout(() => {
        renderActiveFlags();
        renderTimeout = null;
    }, 250);
}

document.addEventListener("DOMContentLoaded", () => {
    initTabs();
    fetchFlags();
    setupSSE();
    setupModal();
});

function initTabs() {
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
            
            e.target.classList.add('active');
            document.getElementById(e.target.dataset.target).classList.add('active');
        });
    });
}

async function fetchFlags() {
    try {
        const resUnack = await fetch('/api/flags?status=unacknowledged');
        const unack = await resUnack.json();
        unack.forEach(f => activeFlagsState.set(f.id, f));
        scheduleRenderFlags();

        const resAck = await fetch('/api/flags?status=acknowledged');
        const ack = await resAck.json();
        ack.forEach(f => auditTrailState.push(f));
        renderAuditTrail();
    } catch (e) {
        console.error("Failed to fetch initial flags", e);
    }
}

function setupSSE() {
    const evtSource = new EventSource('/api/events');
    evtSource.onmessage = (event) => {
        const data = JSON.parse(event.data);
        if (data.type === 'flag_upsert') {
            activeFlagsState.set(data.data.id, data.data);
            scheduleRenderFlags();
        } else if (data.type === 'flag_acknowledged') {
            activeFlagsState.delete(data.data.id);
            auditTrailState.unshift(data.data);
            scheduleRenderFlags();
        } else if (data.type === 'stats_update') {
            updateStatsUI(data.data);
        }
    };
    evtSource.onerror = () => {
        console.error("SSE connection error. Retrying...");
    };
}

function updateStatsUI(stats) {
    const ingestRate = stats.ingest_rate;
    const outputRate = stats.output_rate;
    const totalIngest = stats.total_ingest;
    const totalOutput = stats.total_output;

    document.getElementById('val-ingest-rate').textContent = ingestRate;
    document.getElementById('val-output-rate').textContent = outputRate;
    document.getElementById('val-ingest-total').textContent = totalIngest.toLocaleString();
    document.getElementById('val-output-total').textContent = totalOutput.toLocaleString();

    let reduction = 0;
    if (ingestRate > 0) {
        reduction = ((ingestRate - outputRate) / ingestRate) * 100;
    }
    document.getElementById('val-reduction').textContent = reduction.toFixed(1);

    const AWS_COST_PER_MILLION_MSGS = 1.20; // AWS IoT Core standard messaging
    
    // Use overall average rate for perfectly stable monthly cost projections
    const uptime = stats.uptime || 1;
    const overallIngestRate = totalIngest / uptime;
    const overallOutputRate = totalOutput / uptime;
    
    const monthlyIngestCost = (overallIngestRate * 2592000 / 1000000) * AWS_COST_PER_MILLION_MSGS;
    const monthlyOutputCost = (overallOutputRate * 2592000 / 1000000) * AWS_COST_PER_MILLION_MSGS;

    document.getElementById('val-cost-baseline').textContent = monthlyIngestCost.toFixed(2);
    document.getElementById('val-cost-zaee').textContent = monthlyOutputCost.toFixed(2);

    const totalSavedMsgs = totalIngest - totalOutput;
    const totalSavings = (totalSavedMsgs / 1000000) * AWS_COST_PER_MILLION_MSGS;
    document.getElementById('val-savings-total').textContent = totalSavings.toFixed(4);
}

function renderActiveFlags() {
    const grid = document.getElementById('flags-grid');
    grid.innerHTML = '';
    
    document.getElementById('active-badge').textContent = activeFlagsState.size;

    const sorted = Array.from(activeFlagsState.values()).sort((a, b) => 
        new Date(b.last_detected_at) - new Date(a.last_detected_at)
    );

    sorted.forEach(f => {
        const card = document.createElement('div');
        card.className = 'flag-card glass';
        card.onclick = () => openModal(f);
        
        const title = f.field_name ? `${f.sensor_id} : ${f.field_name}` : f.sensor_id;
        
        card.innerHTML = `
            <div class="flag-type">${f.flag_type.replace(/_/g, ' ')}</div>
            <div class="flag-sensor">${title}</div>
            <div class="flag-msg">${f.message}</div>
            <div class="flag-meta">
                <span>First seen: ${new Date(f.first_detected_at).toLocaleTimeString()}</span>
                <span>Last seen: ${new Date(f.last_detected_at).toLocaleTimeString()}</span>
            </div>
        `;
        grid.appendChild(card);
    });
}

function renderAuditTrail() {
    const tbody = document.getElementById('audit-tbody');
    tbody.innerHTML = '';
    
    auditTrailState.forEach(f => {
        const tr = document.createElement('tr');
        const title = f.field_name ? `${f.sensor_id} : ${f.field_name}` : f.sensor_id;
        
        tr.innerHTML = `
            <td>${new Date(f.acknowledged_at).toLocaleString()}</td>
            <td>${title}</td>
            <td><span class="badge" style="background:var(--accent-green)">${f.flag_type}</span></td>
            <td>${f.acknowledged_by}</td>
            <td>${f.note}</td>
        `;
        tbody.appendChild(tr);
    });
}

function setupModal() {
    document.getElementById('ack-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const id = document.getElementById('ack-flag-id').value;
        const user = document.getElementById('ack-user').value;
        const note = document.getElementById('ack-note').value;
        
        try {
            const res = await fetch(`/api/flags/${id}/acknowledge`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ user, note })
            });
            if (res.ok) {
                closeModal();
            } else {
                const err = await res.json();
                alert(`Failed: ${err.detail}`);
            }
        } catch (e) {
            console.error("Acknowledgment failed", e);
        }
    });
}

function openModal(flag) {
    document.getElementById('ack-flag-id').value = flag.id;
    const title = flag.field_name ? `${flag.sensor_id} : ${flag.field_name}` : flag.sensor_id;
    document.getElementById('modal-flag-info').textContent = `Resolving ${flag.flag_type} for ${title}`;
    
    document.getElementById('ack-user').value = '';
    document.getElementById('ack-note').value = '';
    
    document.getElementById('ack-modal').classList.remove('hidden');
}

function closeModal() {
    document.getElementById('ack-modal').classList.add('hidden');
}
