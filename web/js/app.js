// ==========================================================================
// AO Sphere — Unified Operating System for Modern Business
// Client Application Engine (Minimal & Modern Theme)
// ==========================================================================

const app = {
  activeTab: 'dashboard',
  drawerTab: 'stream',
  theme: 'light',
  apiKey: localStorage.getItem('aosphere_api_key') || 'ao_sk_agent_default_token_12345',
  data: {
    skus: [],
    suppliers: [],
    deals: [],
    invoices: [],
    expenses: [],
    summary: null,
    employees: [],
    shifts: [],
    tasks: [],
    listings: [],
    reviews: [],
    approvals: [],
    auditLogs: [],
    tools: []
  },
  sse: null,

  init() {
    this.setupTheme();
    this.setupShortcuts();
    this.setupSSE();
    this.loadAllData();
    if (window.lucide) {
      lucide.createIcons();
    }
  },

  onApiKeyChange(val) {
    this.apiKey = val.trim();
    localStorage.setItem('aosphere_api_key', this.apiKey);
    this.showToast('Agent API key updated');
  },

  // ---------------- Theme Management ----------------
  setupTheme() {
    const saved = localStorage.getItem('aosphere_theme') || 'light';
    this.setTheme(saved);
  },

  setTheme(theme) {
    this.theme = theme;
    localStorage.setItem('aosphere_theme', theme);
    if (theme === 'dark') {
      document.documentElement.classList.add('theme-dark');
      document.body.className = 'theme-dark bg-slate-900 text-white';
    } else {
      document.documentElement.classList.remove('theme-dark');
      document.body.className = 'theme-light bg-brand-bg text-brand-textMain';
    }
    const icon = document.getElementById('theme-icon');
    if (icon) {
      icon.setAttribute('data-lucide', theme === 'dark' ? 'sun' : 'moon');
      if (window.lucide) lucide.createIcons();
    }
  },

  toggleTheme() {
    this.setTheme(this.theme === 'dark' ? 'light' : 'dark');
  },

  // ---------------- Mobile Responsive Navigation ----------------
  toggleMobileSidebar() {
    const sidebar = document.querySelector('.app-sidebar');
    const backdrop = document.getElementById('sidebar-backdrop');
    if (sidebar) sidebar.classList.toggle('mobile-open');
    if (backdrop) backdrop.classList.toggle('active');
  },

  closeMobileSidebar() {
    const sidebar = document.querySelector('.app-sidebar');
    const backdrop = document.getElementById('sidebar-backdrop');
    if (sidebar) sidebar.classList.remove('mobile-open');
    if (backdrop) backdrop.classList.remove('active');
  },

  // ---------------- Keyboard Shortcuts ----------------
  setupShortcuts() {
    window.addEventListener('keydown', (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        this.openCommandPalette();
      }
      if (e.key === 'Escape') {
        this.closeCommandPalette();
        this.closeModal();
      }
    });
  },

  // ---------------- Server-Sent Events (Live Bus) ----------------
  setupSSE() {
    try {
      this.sse = new EventSource('/api/events');
      const indicator = document.getElementById('sse-indicator');

      this.sse.onopen = () => {
        if (indicator) {
          indicator.className = 'status-pill connected';
          indicator.title = 'Live Event Bus connected (SSE)';
        }
      };

      this.sse.onerror = () => {
        if (indicator) {
          indicator.className = 'status-pill';
          indicator.title = 'Event Bus reconnecting...';
        }
      };

      const eventEntityMap = {
        stock_updated: ['skus', 'summary'],
        sku_created: ['skus'],
        po_created: ['skus', 'approvals', 'summary'],
        po_received: ['skus', 'summary'],
        po_paid: ['summary'],
        deal_created: ['deals', 'summary'],
        deal_stage_changed: ['deals', 'summary'],
        deal_converted: ['deals', 'invoices', 'summary'],
        quote_generated: ['deals'],
        invoice_created: ['invoices', 'summary'],
        shift_updated: ['shifts'],
        task_updated: ['tasks'],
        nap_synced: ['listings'],
        review_replied: ['reviews'],
        approval_decided: ['approvals', 'skus'],
        approval_required: ['approvals']
      };

      Object.keys(eventEntityMap).forEach(evtName => {
        this.sse.addEventListener(evtName, async (e) => {
          this.showToast(`Live Event: ${evtName.replace(/_/g, ' ')}`);
          const entities = eventEntityMap[evtName] || [];
          for (const ent of entities) {
            await this.reloadEntity(ent);
          }
          await this.reloadEntity('auditLogs');
        });
      });
    } catch (err) {
      console.warn('SSE initialization failed', err);
    }
  },

  // ---------------- Granular Entity Reload ----------------
  async reloadEntity(name) {
    const endpoints = {
      skus: '/api/skus',
      suppliers: '/api/suppliers',
      deals: '/api/deals',
      invoices: '/api/invoices',
      summary: '/api/financial-summary',
      employees: '/api/employees',
      shifts: '/api/shifts',
      tasks: '/api/tasks',
      listings: '/api/listings',
      reviews: '/api/reviews',
      approvals: '/api/approvals',
      auditLogs: '/api/audit-logs'
    };
    if (!endpoints[name]) return;
    try {
      const res = await fetch(endpoints[name]);
      if (res.ok) {
        this.data[name] = await res.json();
        this.updateBadges();
        this.render();
        if (name === 'auditLogs') this.renderDrawerTimeline();
      }
    } catch (err) {
      console.warn(`Failed to reload entity ${name}:`, err);
    }
  },

  // ---------------- Full Data Loading (Boot) ----------------
  async loadAllData() {
    try {
      const [skus, suppliers, deals, invoices, summary, employees, shifts, tasks, listings, reviews, approvals, auditLogs] = await Promise.all([
        fetch('/api/skus').then(r => r.json()).catch(() => []),
        fetch('/api/suppliers').then(r => r.json()).catch(() => []),
        fetch('/api/deals').then(r => r.json()).catch(() => []),
        fetch('/api/invoices').then(r => r.json()).catch(() => []),
        fetch('/api/financial-summary').then(r => r.json()).catch(() => null),
        fetch('/api/employees').then(r => r.json()).catch(() => []),
        fetch('/api/shifts').then(r => r.json()).catch(() => []),
        fetch('/api/tasks').then(r => r.json()).catch(() => []),
        fetch('/api/listings').then(r => r.json()).catch(() => []),
        fetch('/api/reviews').then(r => r.json()).catch(() => []),
        fetch('/api/approvals').then(r => r.json()).catch(() => []),
        fetch('/api/audit-logs').then(r => r.json()).catch(() => [])
      ]);

      this.data.skus = skus;
      this.data.suppliers = suppliers;
      this.data.deals = deals;
      this.data.invoices = invoices;
      this.data.summary = summary;
      this.data.employees = employees;
      this.data.shifts = shifts;
      this.data.tasks = tasks;
      this.data.listings = listings;
      this.data.reviews = reviews;
      this.data.approvals = approvals;
      this.data.auditLogs = auditLogs;

      this.updateBadges();
      this.render();
      this.renderDrawerTimeline();
      this.loadMCPTools();
    } catch (err) {
      console.error('Error loading platform data:', err);
    }
  },

  updateBadges() {
    const lowStock = this.data.skus.filter(s => s.stock_quantity <= s.min_threshold);
    const lowStockBadge = document.getElementById('low-stock-count');
    if (lowStockBadge) {
      if (lowStock.length > 0) {
        lowStockBadge.textContent = lowStock.length;
        lowStockBadge.style.display = 'inline-block';
      } else {
        lowStockBadge.style.display = 'none';
      }
    }

    const pendingApprs = this.data.approvals.filter(a => a.status === 'pending');
    const apprBadge = document.getElementById('approval-badge');
    if (apprBadge) {
      apprBadge.textContent = pendingApprs.length;
      apprBadge.style.display = pendingApprs.length > 0 ? 'inline-block' : 'none';
    }
  },

  // ---------------- Navigation ----------------
  setTab(tab) {
    this.activeTab = tab;
    this.closeMobileSidebar();
    document.querySelectorAll('.nav-item').forEach(el => {
      el.classList.toggle('active', el.dataset.tab === tab);
    });
    this.render();
  },

  render() {
    const main = document.getElementById('main-content');
    if (!main) return;

    switch (this.activeTab) {
      case 'dashboard':
        main.innerHTML = this.renderDashboard();
        break;
      case 'erp':
        main.innerHTML = this.renderERP();
        break;
      case 'crm':
        main.innerHTML = this.renderCRM();
        this.setupKanbanDragDrop();
        break;
      case 'finance':
        main.innerHTML = this.renderFinance();
        break;
      case 'hr':
        main.innerHTML = this.renderHR();
        break;
      case 'tasks':
        main.innerHTML = this.renderTasks();
        break;
      case 'presence':
        main.innerHTML = this.renderPresence();
        break;
      case 'reviews':
        main.innerHTML = this.renderReviews();
        break;
      case 'approvals':
        main.innerHTML = this.renderApprovals();
        break;
      case 'audit':
        main.innerHTML = this.renderAudit();
        break;
      default:
        main.innerHTML = `<div class="p-8">Select a module from the sidebar</div>`;
    }

    if (window.lucide) {
      lucide.createIcons();
    }
  },

  // ---------------- Module 1: Dashboard ----------------
  renderDashboard() {
    const s = this.data.summary || { total_revenue: 0, total_expense: 0, net_profit: 0, unpaid_total: 0 };
    const lowStock = this.data.skus.filter(item => item.stock_quantity <= item.min_threshold);
    const pendingApprs = this.data.approvals.filter(a => a.status === 'pending');
    const pipelineTotal = this.data.deals.reduce((acc, d) => acc + (d.stage !== 'lost' ? d.value : 0), 0);

    return `
      <!-- Ambient Ethereal Blobs (Matching Hero Page) -->
      <div class="bg-blob w-[500px] h-[500px] bg-blue-100 top-[-10%] right-[-5%]"></div>
      <div class="bg-blob w-[450px] h-[450px] bg-slate-200/60 bottom-[0%] left-[-5%]"></div>

      <div class="page-header">
        <div>
          <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-blue-100 text-brand-blue text-[11px] font-bold uppercase tracking-widest mb-3">
            <i data-lucide="network" class="w-3.5 h-3.5"></i> Unified OS Workspace
          </div>
          <h1 class="page-title">Executive Overview</h1>
          <p class="page-sub">Connect your operations, sales, finances, and team in a single, minimal workspace.</p>
        </div>
        <div class="page-actions">
          <button class="btn-press btn btn-secondary" onclick="app.setTab('approvals')">
            <i data-lucide="shield-check" class="w-4 h-4 text-amber-500"></i>
            Approvals (${pendingApprs.length})
          </button>
          <button class="btn-press btn btn-primary" onclick="app.openCommandPalette()">
            <i data-lucide="search" class="w-4 h-4"></i>
            Command Bar (Ctrl+K)
          </button>
        </div>
      </div>

      <!-- Telemetry Cards (Matching Dashboard Mockup Cards) -->
      <div class="stats-grid">
        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-3">
            <div class="stat-title">Cash Revenue</div>
            <div class="w-10 h-10 bg-blue-50 rounded-xl flex items-center justify-center">
              <i data-lucide="trending-up" class="w-5 h-5 text-brand-blue"></i>
            </div>
          </div>
          <div class="stat-value">$${s.total_revenue.toLocaleString('en-US', { minimumFractionDigits: 2 })}</div>
          <div class="stat-trend trend-up">
            <i data-lucide="check" class="w-3.5 h-3.5 inline"></i> Paid Invoices (${s.paid_count || 0})
          </div>
        </div>

        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-3">
            <div class="stat-title">Net Profit (P&L)</div>
            <div class="w-10 h-10 bg-emerald-50 rounded-xl flex items-center justify-center">
              <i data-lucide="shopping-cart" class="w-5 h-5 text-emerald-500"></i>
            </div>
          </div>
          <div class="stat-value" style="color: #059669;">$${s.net_profit.toLocaleString('en-US', { minimumFractionDigits: 2 })}</div>
          <div class="stat-trend trend-up">
            Expenses: $${s.total_expense.toLocaleString('en-US', { minimumFractionDigits: 2 })}
          </div>
        </div>

        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-3">
            <div class="stat-title">Active Deals Pipeline</div>
            <div class="w-10 h-10 bg-purple-50 rounded-xl flex items-center justify-center">
              <i data-lucide="users" class="w-5 h-5 text-purple-500"></i>
            </div>
          </div>
          <div class="stat-value">$${pipelineTotal.toLocaleString('en-US', { minimumFractionDigits: 2 })}</div>
          <div class="stat-trend text-purple-600 font-semibold">
            ${this.data.deals.length} Active Opportunities
          </div>
        </div>

        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-3">
            <div class="stat-title">Threshold Alerts</div>
            <div class="w-10 h-10 ${lowStock.length > 0 ? 'bg-amber-50' : 'bg-slate-50'} rounded-xl flex items-center justify-center">
              <i data-lucide="alert-triangle" class="w-5 h-5 ${lowStock.length > 0 ? 'text-amber-500' : 'text-slate-400'}"></i>
            </div>
          </div>
          <div class="stat-value" style="color: ${lowStock.length > 0 ? '#D97706' : 'var(--text-main)'}">
            ${lowStock.length} SKUs
          </div>
          <div class="stat-trend ${lowStock.length > 0 ? 'trend-amber' : 'text-slate-400'}">
            ${lowStock.length > 0 ? '⚠️ Automated reorders triggered' : '✓ All levels nominal'}
          </div>
        </div>
      </div>

      <!-- Two-Column Breakdown (Matching Mockup Containers) -->
      <div style="display: grid; grid-template-columns: 1.1fr 0.9fr; gap: 24px; position: relative; z-index: 10;">
        
        <!-- Inventory Watchlist Card -->
        <div class="table-container mockup-item" style="padding: 24px; margin-bottom: 0;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 18px; border-bottom: 1px solid #F1F5F9; padding-bottom: 12px;">
            <div>
              <h3 style="font-size: 17px; font-weight: 800; color: #0F172A;">Critical Inventory Watchlist</h3>
              <p style="font-size: 12px; color: #94A3B8; font-weight: 500; margin-top: 2px;">Real-time stock threshold monitors</p>
            </div>
            <button class="btn-press btn btn-secondary btn-sm" onclick="app.setTab('erp')">
              Catalog <i data-lucide="arrow-right" class="w-3.5 h-3.5"></i>
            </button>
          </div>
          ${lowStock.length === 0 ? '<p style="font-size:13px; color:var(--text-muted); padding:12px 0;">All inventory stocks are within safe operational limits.</p>' : `
            <table class="data-table">
              <thead>
                <tr>
                  <th>Product</th>
                  <th>SKU</th>
                  <th>Stock</th>
                  <th>Status</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                ${lowStock.map(i => `
                  <tr>
                    <td>
                      <div class="flex items-center gap-3">
                        <div class="w-8 h-8 rounded-lg bg-amber-50 flex items-center justify-center shrink-0">
                          <i data-lucide="package" class="w-4 h-4 text-amber-600"></i>
                        </div>
                        <span class="font-bold text-brand-textMain text-[13.5px]">${i.name}</span>
                      </div>
                    </td>
                    <td><span class="mono-code">${i.code}</span></td>
                    <td><span style="color:#D97706; font-weight:800;">${i.stock_quantity}</span> / <small class="text-slate-400 font-medium">Min ${i.min_threshold}</small></td>
                    <td><span class="bg-amber-100 text-amber-700 text-[11px] font-bold px-2.5 py-1 rounded-md">Low Stock</span></td>
                    <td>
                      <button class="btn-press btn btn-secondary btn-sm" onclick="app.openStockModal('${i.id}', '${i.code}', ${i.stock_quantity})">Adjust</button>
                    </td>
                  </tr>
                `).join('')}
              </tbody>
            </table>
          `}
        </div>

        <!-- Activity Stream Card -->
        <div class="table-container mockup-item" style="padding: 24px; margin-bottom: 0;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 18px; border-bottom: 1px solid #F1F5F9; padding-bottom: 12px;">
            <div>
              <h3 style="font-size: 17px; font-weight: 800; color: #0F172A;">Dual-Actor Activity Stream</h3>
              <p style="font-size: 12px; color: #94A3B8; font-weight: 500; margin-top: 2px;">Autonomous Agent & Human actions</p>
            </div>
            <button class="btn-press btn btn-secondary btn-sm" onclick="app.setTab('audit')">
              Audit <i data-lucide="arrow-right" class="w-3.5 h-3.5"></i>
            </button>
          </div>
          <div style="display: flex; flex-direction: column; gap: 10px;">
            ${this.data.auditLogs.slice(0, 5).map(l => `
              <div style="padding: 12px 14px; background: #F8FAFC; border: 1px solid var(--border-subtle); border-radius: 12px;">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px;">
                  <span class="actor-pill actor-${l.actor_type}">${l.actor_type.toUpperCase()}: ${l.actor_id}</span>
                  <span style="color: var(--text-dim); font-size: 10px; font-family: var(--font-mono);">${new Date(l.created_at).toLocaleTimeString()}</span>
                </div>
                <div style="font-size: 12.5px; color: var(--text-main);">
                  <strong style="color: #2563EB;">${l.action}</strong>: ${l.reasoning}
                </div>
              </div>
            `).join('')}
          </div>
        </div>

      </div>
    `;
  },

  // ---------------- Module 2: ERP & Inventory ----------------
  renderERP() {
    return `
      <div class="page-header">
        <div>
          <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-blue-100 text-brand-blue text-[11px] font-bold uppercase tracking-widest mb-3">
            <i data-lucide="network" class="w-3.5 h-3.5"></i> Core Infrastructure
          </div>
          <h1 class="page-title">Enterprise Resource Planning</h1>
          <p class="page-sub">Centralize supply chain, SKU inventories, and automated threshold reorder triggers.</p>
        </div>
        <div class="page-actions">
          <button class="btn-press btn btn-secondary" onclick="app.openNewSKUModal()">
            <i data-lucide="package-plus" class="w-4 h-4 text-brand-blue"></i> Add SKU
          </button>
          <button class="btn-press btn btn-primary" onclick="app.openNewPOModal()">
            <i data-lucide="plus" class="w-4 h-4"></i> New Purchase Order
          </button>
        </div>
      </div>

      <div class="table-container mockup-item">
        <table class="data-table">
          <thead>
            <tr>
              <th>Product</th>
              <th>SKU Code</th>
              <th>Category</th>
              <th>Stock Level</th>
              <th>Min Threshold</th>
              <th>Unit Cost</th>
              <th>Selling Price</th>
              <th>Supplier</th>
              <th>Quick Actions</th>
            </tr>
          </thead>
          <tbody>
            ${this.data.skus.map(item => {
              const isLow = item.stock_quantity <= item.min_threshold;
              const isOut = item.stock_quantity === 0;
              return `
                <tr>
                  <td>
                    <div class="flex items-center gap-3">
                      <div class="w-9 h-9 rounded-xl ${isLow ? 'bg-amber-50 text-amber-600' : 'bg-blue-50 text-brand-blue'} flex items-center justify-center shrink-0">
                        <i data-lucide="package" class="w-4 h-4"></i>
                      </div>
                      <span class="font-bold text-brand-textMain text-[14px]">${item.name}</span>
                    </div>
                  </td>
                  <td><span class="mono-code">${item.code}</span></td>
                  <td><span class="badge" style="background:#F1F5F9; color:#475569;">${item.category}</span></td>
                  <td>
                    <div style="display:flex; align-items:center; gap:8px;">
                      <span style="font-weight:800; font-size:14px; ${isLow ? 'color:#D97706' : 'color:#0F172A'}">${item.stock_quantity}</span>
                      ${isOut ? '<span class="bg-red-100 text-red-700 text-[11px] font-bold px-2 py-0.5 rounded-md">Out of Stock</span>' :
                        isLow ? '<span class="bg-amber-100 text-amber-700 text-[11px] font-bold px-2 py-0.5 rounded-md">Low Stock</span>' :
                        '<span class="bg-emerald-100 text-emerald-700 text-[11px] font-bold px-2 py-0.5 rounded-md">In Stock</span>'}
                    </div>
                  </td>
                  <td class="font-medium text-slate-500">${item.min_threshold}</td>
                  <td class="font-semibold text-slate-700">$${item.unit_cost.toFixed(2)}</td>
                  <td class="font-bold text-brand-blue">$${item.selling_price.toFixed(2)}</td>
                  <td class="text-slate-500 font-medium">${item.supplier_name || 'Direct'}</td>
                  <td>
                    <div style="display:flex; gap:6px;">
                      <button class="btn-press btn btn-secondary btn-sm" onclick="app.quickAdjustStock('${item.id}', 10, 'Received restock batch')">+10</button>
                      <button class="btn-press btn btn-secondary btn-sm" onclick="app.quickAdjustStock('${item.id}', -5, 'Order fulfillment test')">-5</button>
                      <button class="btn-press btn btn-secondary btn-sm" onclick="app.openStockModal('${item.id}', '${item.code}', ${item.stock_quantity})">Custom</button>
                    </div>
                  </td>
                </tr>
              `;
            }).join('')}
          </tbody>
        </table>
      </div>
    `;
  },

  // ---------------- Module 3: CRM & Deals Visual Pipeline ----------------
  renderCRM() {
    const stages = [
      { id: 'lead', name: 'New Leads' },
      { id: 'qualified', name: 'Qualified' },
      { id: 'proposal', name: 'Proposal' },
      { id: 'negotiation', name: 'Negotiation' },
      { id: 'won', name: 'Won' }
    ];

    return `
      <div class="page-header">
        <div>
          <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-100 text-emerald-700 text-[11px] font-bold uppercase tracking-widest mb-3">
            <i data-lucide="users" class="w-3.5 h-3.5"></i> Growth Engine
          </div>
          <h1 class="page-title">Customer & Sales Pipeline</h1>
          <p class="page-sub">Drag and drop deals across stages, track probability and customer relationships.</p>
        </div>
        <div class="page-actions">
          <button class="btn-press btn btn-primary" onclick="app.openNewDealModal()">
            <i data-lucide="plus" class="w-4 h-4"></i> Add New Deal
          </button>
        </div>
      </div>

      <div class="kanban-board">
        ${stages.map(stage => {
          const deals = this.data.deals.filter(d => d.stage === stage.id);
          const colTotal = deals.reduce((sum, d) => sum + d.value, 0);
          return `
            <div class="kanban-column" data-stage="${stage.id}">
              <div class="kanban-col-header">
                <div>
                  <span class="col-title">${stage.name}</span>
                  <div style="font-size:12px; color:var(--text-muted); font-weight:600; margin-top:2px;">$${colTotal.toLocaleString()}</div>
                </div>
                <span class="col-badge">${deals.length}</span>
              </div>
              <div class="kanban-cards-area" id="col-${stage.id}">
                ${deals.map(deal => `
                  <div class="deal-card" draggable="true" data-deal-id="${deal.id}">
                    <div class="deal-title">${deal.title}</div>
                    <div class="deal-customer">${deal.customer_name || 'Client'} • ${deal.customer_company || ''}</div>
                    <div class="deal-footer">
                      <span class="deal-value">$${deal.value.toLocaleString()}</span>
                      <span style="font-size:11px; font-weight:700; color:#64748B; background:var(--bg-tertiary); padding:2px 6px; border-radius:4px;">${deal.probability}%</span>
                    </div>
                    <button class="btn-press btn btn-secondary btn-sm" style="margin-top:10px; width:100%; font-size:11px; height:28px;" onclick="event.stopPropagation(); app.convertDealToInvoice('${deal.id}')">
                      <i data-lucide="receipt" class="w-3.5 h-3.5 text-blue-500"></i> Convert to Invoice
                    </button>
                  </div>
                `).join('')}
              </div>
            </div>
          `;
        }).join('')}
      </div>
    `;
  },

  setupKanbanDragDrop() {
    const cards = document.querySelectorAll('.deal-card');
    const cols = document.querySelectorAll('.kanban-column');

    cards.forEach(card => {
      card.addEventListener('dragstart', (e) => {
        e.dataTransfer.setData('text/plain', card.dataset.dealId);
        card.style.opacity = '0.5';
      });
      card.addEventListener('dragend', () => {
        card.style.opacity = '1';
      });
    });

    cols.forEach(col => {
      col.addEventListener('dragover', (e) => {
        e.preventDefault();
        col.style.borderColor = 'var(--brand-blue)';
      });
      col.addEventListener('dragleave', () => {
        col.style.borderColor = 'var(--border-subtle)';
      });
      col.addEventListener('drop', async (e) => {
        e.preventDefault();
        col.style.borderColor = 'var(--border-subtle)';
        const dealId = e.dataTransfer.getData('text/plain');
        const targetStage = col.dataset.stage;
        if (dealId && targetStage) {
          await fetch('/api/deals/stage', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ deal_id: dealId, stage: targetStage })
          });
          await app.reloadEntity('deals');
          await app.reloadEntity('summary');
          await app.reloadEntity('auditLogs');
        }
      });
    });
  },

  // ---------------- Module 4: Finance & Invoicing ----------------
  renderFinance() {
    const s = this.data.summary || { total_revenue: 0, total_expense: 0, net_profit: 0, unpaid_total: 0 };
    return `
      <div class="page-header">
        <div>
          <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-purple-100 text-purple-700 text-[11px] font-bold uppercase tracking-widest mb-3">
            <i data-lucide="pie-chart" class="w-3.5 h-3.5"></i> Stability & Capital
          </div>
          <h1 class="page-title">Financial & Administrative</h1>
          <p class="page-sub">Oversee automated P&L statements, tax compliance invoices, and client receivables.</p>
        </div>
        <div class="page-actions">
          <button class="btn-press btn btn-primary" onclick="app.openNewInvoiceModal()">
            <i data-lucide="plus" class="w-4 h-4"></i> Create Invoice
          </button>
        </div>
      </div>

      <div class="stats-grid">
        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-2">
            <div class="stat-title">Cash Revenue</div>
            <div class="w-10 h-10 bg-blue-50 rounded-xl flex items-center justify-center">
              <i data-lucide="trending-up" class="w-5 h-5 text-brand-blue"></i>
            </div>
          </div>
          <div class="stat-value">$${s.total_revenue.toLocaleString('en-US', { minimumFractionDigits: 2 })}</div>
          <div class="stat-trend trend-up">✓ Collected</div>
        </div>

        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-2">
            <div class="stat-title">Operating Expenses</div>
            <div class="w-10 h-10 bg-purple-50 rounded-xl flex items-center justify-center">
              <i data-lucide="pie-chart" class="w-5 h-5 text-purple-600"></i>
            </div>
          </div>
          <div class="stat-value">$${s.total_expense.toLocaleString('en-US', { minimumFractionDigits: 2 })}</div>
          <div class="stat-trend text-slate-500 font-medium">Procurement & Cloud</div>
        </div>

        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-2">
            <div class="stat-title">Net Profit (P&L)</div>
            <div class="w-10 h-10 bg-emerald-50 rounded-xl flex items-center justify-center">
              <i data-lucide="check" class="w-5 h-5 text-emerald-500"></i>
            </div>
          </div>
          <div class="stat-value" style="color:#059669;">$${s.net_profit.toLocaleString('en-US', { minimumFractionDigits: 2 })}</div>
          <div class="stat-trend trend-up">P&L Margin: Healthy</div>
        </div>

        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-2">
            <div class="stat-title">Pending Receivables</div>
            <div class="w-10 h-10 bg-amber-50 rounded-xl flex items-center justify-center">
              <i data-lucide="clock" class="w-5 h-5 text-amber-500"></i>
            </div>
          </div>
          <div class="stat-value" style="color:#D97706;">$${s.unpaid_total.toLocaleString('en-US', { minimumFractionDigits: 2 })}</div>
          <div class="stat-trend trend-amber">Uncollected Due</div>
        </div>
      </div>

      <div class="table-container mockup-item">
        <div style="padding:18px 24px; font-weight:800; font-size:16px; border-bottom:1px solid #F1F5F9; display:flex; justify-content:space-between; align-items:center;">
          <span>Recent Invoices</span>
          <span class="text-[12px] text-slate-400 font-medium">Auto-generated via Billing Engine</span>
        </div>
        <table class="data-table">
          <thead>
            <tr>
              <th>Invoice #</th>
              <th>Customer</th>
              <th>Issue Date</th>
              <th>Due Date</th>
              <th>Subtotal</th>
              <th>Tax (8%)</th>
              <th>Total Amount</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            ${this.data.invoices.map(inv => `
              <tr>
                <td><span class="mono-code">${inv.invoice_number}</span></td>
                <td><strong>${inv.customer_name || 'Client'}</strong></td>
                <td class="text-slate-500">${inv.issue_date}</td>
                <td class="text-slate-500">${inv.due_date}</td>
                <td class="font-medium">$${inv.subtotal.toFixed(2)}</td>
                <td class="text-slate-400">$${inv.tax_amount.toFixed(2)}</td>
                <td><strong style="color:#059669; font-size:14px;">$${inv.total_amount.toFixed(2)}</strong></td>
                <td>
                  <span class="badge" style="background:${inv.status === 'paid' ? '#ECFDF5' : '#FEF3C7'}; color:${inv.status === 'paid' ? '#047857' : '#B45309'}; border:1px solid ${inv.status === 'paid' ? '#A7F3D0' : '#FDE68A'};">
                    ${inv.status.toUpperCase()}
                  </span>
                </td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    `;
  },

  // ---------------- Module 5: HRMS & Shifts ----------------
  renderHR() {
    return `
      <div class="page-header">
        <div>
          <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-orange-100 text-orange-700 text-[11px] font-bold uppercase tracking-widest mb-3">
            <i data-lucide="zap" class="w-3.5 h-3.5"></i> Execution & People
          </div>
          <h1 class="page-title">Shifts & Team Records</h1>
          <p class="page-sub">Coordinate employee shift schedules, digital timesheets, and wage rates in one place.</p>
        </div>
      </div>

      <div style="display:grid; grid-template-columns: 1.8fr 1.2fr; gap:24px;">
        
        <!-- Shifts Schedule Grid -->
        <div class="table-container mockup-item">
          <div style="padding:18px 24px; font-weight:800; font-size:16px; border-bottom:1px solid #F1F5F9;">
            Today's Shift Roster
          </div>
          <table class="data-table">
            <thead>
              <tr>
                <th>Employee</th>
                <th>Time Window</th>
                <th>Station</th>
                <th>Status</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              ${this.data.shifts.map(sh => `
                <tr>
                  <td><strong>${sh.employee_name}</strong></td>
                  <td class="font-medium text-slate-500">${sh.start_time} — ${sh.end_time}</td>
                  <td><span class="badge" style="background:#F1F5F9; color:#475569;">${sh.station}</span></td>
                  <td>
                    <span class="badge" style="background:${sh.status === 'clocked_in' ? '#ECFDF5' : '#EFF6FF'}; color:${sh.status === 'clocked_in' ? '#047857' : '#1D4ED8'}; border:1px solid ${sh.status === 'clocked_in' ? '#A7F3D0' : '#BFDBFE'};">
                      ${sh.status.replace('_', ' ').toUpperCase()}
                    </span>
                  </td>
                  <td>
                    <button class="btn-press btn btn-secondary btn-sm" onclick="app.toggleShiftClock('${sh.id}', '${sh.status === 'clocked_in' ? 'completed' : 'clocked_in'}')">
                      ${sh.status === 'clocked_in' ? 'Clock Out' : 'Clock In'}
                    </button>
                  </td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>

        <!-- Employee Directory -->
        <div class="table-container mockup-item" style="padding:24px;">
          <h3 style="font-size:16px; font-weight:800; margin-bottom:16px; color:#0F172A;">Active Employees</h3>
          <div style="display:flex; flex-direction:column; gap:12px;">
            ${this.data.employees.map(emp => `
              <div style="display:flex; align-items:center; justify-content:space-between; padding:12px 14px; background:#F8FAFC; border:1px solid var(--border-subtle); border-radius:12px;">
                <div>
                  <div style="font-weight:700; font-size:14px; color:#0F172A;">${emp.name}</div>
                  <div style="font-size:12px; color:#64748B; font-weight:500;">${emp.role} • ${emp.department}</div>
                </div>
                <div style="font-family:var(--font-mono); font-size:13px; font-weight:700; color:#2563EB; background:#EFF6FF; padding:3px 8px; border-radius:6px;">$${emp.hourly_rate.toFixed(0)}/hr</div>
              </div>
            `).join('')}
          </div>
        </div>

      </div>
    `;
  },

  // ---------------- Module 6: Project Tasks ----------------
  renderTasks() {
    return `
      <div class="page-header">
        <div>
          <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-orange-100 text-orange-700 text-[11px] font-bold uppercase tracking-widest mb-3">
            <i data-lucide="check-square" class="w-3.5 h-3.5"></i> Execution
          </div>
          <h1 class="page-title">Tasks & Delivery Milestones</h1>
          <p class="page-sub">Track cross-team delivery priorities and employee task ownership.</p>
        </div>
      </div>

      <div class="table-container mockup-item">
        <table class="data-table">
          <thead>
            <tr>
              <th>Task</th>
              <th>Project</th>
              <th>Assignee</th>
              <th>Priority</th>
              <th>Due Date</th>
              <th>Status</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            ${this.data.tasks.map(t => `
              <tr>
                <td><strong>${t.title}</strong><br><small style="color:var(--text-muted); font-weight:500;">${t.description}</small></td>
                <td><span class="badge" style="background:#F1F5F9; color:#475569;">${t.project_name}</span></td>
                <td class="font-medium text-slate-600">${t.assignee_name}</td>
                <td>
                  <span class="badge" style="background:${t.priority === 'urgent' ? '#FEF2F2' : '#EFF6FF'}; color:${t.priority === 'urgent' ? '#B91C1C' : '#1D4ED8'}; border:1px solid ${t.priority === 'urgent' ? '#FECACA' : '#BFDBFE'};">
                    ${t.priority.toUpperCase()}
                  </span>
                </td>
                <td class="font-medium text-slate-500">${t.due_date}</td>
                <td><span class="badge" style="background:#F1F5F9; color:#334155;">${t.status.toUpperCase()}</span></td>
                <td>
                  <button class="btn-press btn btn-secondary btn-sm" onclick="app.cycleTaskStatus('${t.id}', '${t.status}')">
                    Advance
                  </button>
                </td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    `;
  },

  // ---------------- Module 7: Local Listings (NAP) ----------------
  renderPresence() {
    const first = this.data.listings[0] || {};
    return `
      <div class="page-header">
        <div>
          <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-indigo-100 text-indigo-700 text-[11px] font-bold uppercase tracking-widest mb-3">
            <i data-lucide="globe-2" class="w-3.5 h-3.5"></i> Brand Control
          </div>
          <h1 class="page-title">Local Listing Sync (NAP)</h1>
          <p class="page-sub">Automatically update Name, Address, Phone data across search engines and directories.</p>
        </div>
        <div class="page-actions">
          <button class="btn-press btn btn-primary" onclick="app.syncAllNAP()">
            <i data-lucide="refresh-cw" class="w-4 h-4"></i> Sync All Directories
          </button>
        </div>
      </div>

      <!-- Sync Matrix -->
      <div class="stats-grid">
        ${this.data.listings.map(l => `
          <div class="stat-card mockup-item">
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:8px;">
              <span style="font-weight:800; font-size:15px; color:#0F172A;">${l.directory}</span>
              <span class="badge" style="background:${l.sync_status === 'synced' ? '#ECFDF5' : '#FEF3C7'}; color:${l.sync_status === 'synced' ? '#047857' : '#B45309'}; border:1px solid ${l.sync_status === 'synced' ? '#A7F3D0' : '#FDE68A'};">
                ${l.sync_status.toUpperCase()}
              </span>
            </div>
            <div style="font-size:12.5px; color:var(--text-muted); margin-top:6px; line-height:1.4;">
              <div><strong>Name:</strong> ${l.nap_business_name}</div>
              <div><strong>Phone:</strong> ${l.nap_phone}</div>
              <div style="margin-top:6px; font-size:10.5px; color:#94A3B8;">Last Synced: ${new Date(l.last_synced_at).toLocaleDateString()}</div>
            </div>
          </div>
        `).join('')}
      </div>

      <!-- Edit NAP profile -->
      <div class="table-container mockup-item" style="padding:28px; max-width:620px;">
        <h3 style="font-size:17px; font-weight:800; margin-bottom:18px; color:#0F172A;">Master Business Info (NAP)</h3>
        <div style="display:flex; flex-direction:column; gap:14px;">
          <div>
            <label style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase; letter-spacing:0.04em;">Business Name</label>
            <input type="text" id="nap-name" class="form-input" style="margin-top:6px;" value="${first.nap_business_name || 'AOSphere Technologies Ltd'}">
          </div>
          <div>
            <label style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase; letter-spacing:0.04em;">Physical Address</label>
            <input type="text" id="nap-address" class="form-input" style="margin-top:6px;" value="${first.nap_address || '742 Innovation Blvd, Suite 400, Austin TX 78701'}">
          </div>
          <div>
            <label style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase; letter-spacing:0.04em;">Phone Number</label>
            <input type="text" id="nap-phone" class="form-input" style="margin-top:6px;" value="${first.nap_phone || '+1 (512) 555-0199'}">
          </div>
          <div>
            <label style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase; letter-spacing:0.04em;">Official Website</label>
            <input type="text" id="nap-website" class="form-input" style="margin-top:6px;" value="${first.nap_website || 'https://aosphere.example.com'}">
          </div>
          <button class="btn-press btn btn-primary" style="margin-top:8px;" onclick="app.saveAndSyncNAP()">
            <i data-lucide="radio" class="w-4 h-4"></i> Broadcast to All Search Engines
          </button>
        </div>
      </div>
    `;
  },

  // ---------------- Module 8: Reviews & Reputation ----------------
  renderReviews() {
    return `
      <div class="page-header">
        <div>
          <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-indigo-100 text-indigo-700 text-[11px] font-bold uppercase tracking-widest mb-3">
            <i data-lucide="star" class="w-3.5 h-3.5"></i> Reputation Management
          </div>
          <h1 class="page-title">Reviews & Sentiment Intelligence</h1>
          <p class="page-sub">Aggregate incoming client feedback, track sentiment trends, and authorize AI-generated replies.</p>
        </div>
      </div>

      <!-- Reputation Score Hero Widget (Matching Section 5 Mockup) -->
      <div class="table-container mockup-item" style="padding: 24px; margin-bottom: 24px;">
        <div class="flex justify-between items-center">
          <div>
            <h3 class="font-extrabold text-[18px] text-brand-textMain">Reputation Score</h3>
            <div class="flex items-center gap-2 mt-1">
              <div class="flex text-amber-400">
                <i data-lucide="star" class="w-4 h-4 fill-amber-400"></i>
                <i data-lucide="star" class="w-4 h-4 fill-amber-400"></i>
                <i data-lucide="star" class="w-4 h-4 fill-amber-400"></i>
                <i data-lucide="star" class="w-4 h-4 fill-amber-400"></i>
                <i data-lucide="star-half" class="w-4 h-4 fill-amber-400"></i>
              </div>
              <span class="font-extrabold text-[16px] text-brand-textMain">4.8</span>
              <span class="text-[12px] text-slate-400 font-medium">(1,204 Reviews)</span>
            </div>
          </div>
          <div class="bg-indigo-50 border border-indigo-100 text-indigo-600 text-[12px] font-bold px-4 py-2 rounded-xl flex items-center gap-1.5 shadow-sm">
            <i data-lucide="trending-up" class="w-4 h-4"></i> +5.2% this quarter
          </div>
        </div>
      </div>

      <div>
        ${this.data.reviews.map(rev => `
          <div class="review-item-card mockup-item">
            <div class="review-meta">
              <div class="author-rating">
                <span style="color:#0F172A; font-weight:800;">${rev.author_name}</span>
                <span class="stars">${'★'.repeat(rev.rating)}${'☆'.repeat(5 - rev.rating)}</span>
                <span style="font-size:12px; color:var(--text-muted); font-weight:500;">on ${rev.platform}</span>
              </div>
              <span class="badge-sentiment badge-${rev.sentiment}">
                ${rev.sentiment.toUpperCase()}
              </span>
            </div>
            <p style="font-size:14px; margin: 10px 0; color:#334155; line-height:1.5;">${rev.review_text}</p>
            
            ${rev.reply_text ? `
              <div class="review-reply-box">
                <div style="font-weight:700; font-size:12px; color:#7E22CE; margin-bottom:4px; display:flex; align-items:center; gap:6px;">
                  <i data-lucide="bot" class="w-3.5 h-3.5"></i>
                  ${rev.reply_status === 'drafted_by_agent' ? 'Agent-Drafted Reply (Pending Publish)' : 'Official Published Reply'}
                </div>
                <div style="color:#334155; font-size:13px;">${rev.reply_text}</div>
                ${rev.reply_status === 'drafted_by_agent' ? `
                  <div style="margin-top:10px; display:flex; gap:8px;">
                    <button class="btn-press btn btn-primary btn-sm" onclick="app.publishReviewReply('${rev.id}')">
                      Publish to ${rev.platform}
                    </button>
                  </div>
                ` : ''}
              </div>
            ` : `
              <button class="btn-press btn btn-secondary btn-sm" style="margin-top:8px;" onclick="app.draftAIReply('${rev.id}')">
                <i data-lucide="sparkles" class="w-3.5 h-3.5 text-purple-600"></i> Generate AI Response
              </button>
            `}
          </div>
        `).join('')}
      </div>
    `;
  },

  // ---------------- Module 9: Approvals (HITL) ----------------
  renderApprovals() {
    return `
      <div class="page-header">
        <div>
          <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-amber-100 text-amber-800 text-[11px] font-bold uppercase tracking-widest mb-3">
            <i data-lucide="shield-check" class="w-3.5 h-3.5"></i> Governance & Safety
          </div>
          <h1 class="page-title">Human-in-the-Loop (HITL) Approvals</h1>
          <p class="page-sub">Safety gates for high-value purchase orders, payroll disbursements, and autonomous actions.</p>
        </div>
      </div>

      <div style="display:flex; flex-direction:column; gap:16px;">
        ${this.data.approvals.length === 0 ? '<div class="table-container mockup-item" style="padding:28px;">Inbox Zero. No pending authorizations.</div>' : ''}
        ${this.data.approvals.map(appr => `
          <div class="table-container mockup-item" style="padding:24px; border-left:4px solid ${appr.status === 'pending' ? '#F59E0B' : '#10B981'};">
            <div style="display:flex; justify-content:space-between; align-items:flex-start;">
              <div>
                <span class="actor-pill actor-${appr.requested_by_type}">${appr.requested_by_type.toUpperCase()}: ${appr.requested_by_id}</span>
                <h3 style="font-size:17px; font-weight:800; margin:8px 0 4px 0; color:#0F172A;">${appr.title}</h3>
                <p style="font-size:13.5px; color:#64748B;">${appr.description}</p>
                <div style="margin-top:12px; font-family:var(--font-mono); font-size:11.5px; background:#F8FAFC; border:1px solid var(--border-subtle); padding:10px 14px; border-radius:8px; color:#334155;">
                  ${appr.payload_json}
                </div>
              </div>
              <div>
                ${appr.status === 'pending' ? `
                  <div style="display:flex; gap:8px;">
                    <button class="btn-press btn btn-primary btn-sm" onclick="app.decideApproval('${appr.id}', 'approved')">
                      <i data-lucide="check" class="w-3.5 h-3.5"></i> Authorize
                    </button>
                    <button class="btn-press btn btn-secondary btn-sm" onclick="app.decideApproval('${appr.id}', 'rejected')">
                      Decline
                    </button>
                  </div>
                ` : `
                  <span class="badge" style="background:#ECFDF5; color:#047857; border:1px solid #A7F3D0; font-size:12px; padding:4px 12px; border-radius:9999px;">
                    ✓ ${appr.status.toUpperCase()} by ${appr.decided_by || 'Admin'}
                  </span>
                `}
              </div>
            </div>
          </div>
        `).join('')}
      </div>
    `;
  },

  // ---------------- Module 10: Dual-Actor Audit ----------------
  renderAudit() {
    return `
      <div class="page-header">
        <div>
          <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-slate-200 text-slate-700 text-[11px] font-bold uppercase tracking-widest mb-3">
            <i data-lucide="file-text" class="w-3.5 h-3.5"></i> Provenance Ledger
          </div>
          <h1 class="page-title">Dual-Actor Audit Trail</h1>
          <p class="page-sub">Cryptographically reliable provenance tracking for every Human and Autonomous Agent action.</p>
        </div>
      </div>

      <div class="table-container mockup-item">
        <table class="data-table">
          <thead>
            <tr>
              <th>Timestamp</th>
              <th>Actor</th>
              <th>Action</th>
              <th>Target Entity</th>
              <th>Reasoning / Provenance</th>
            </tr>
          </thead>
          <tbody>
            ${this.data.auditLogs.map(l => `
              <tr>
                <td style="font-family:var(--font-mono); font-size:11px; color:var(--text-dim);">${new Date(l.created_at).toLocaleString()}</td>
                <td>
                  <span class="actor-pill actor-${l.actor_type}">
                    ${l.actor_type.toUpperCase()}: ${l.actor_id}
                  </span>
                </td>
                <td><strong style="color:#0F172A;">${l.action}</strong></td>
                <td><span class="mono-code">${l.entity_type}:${l.entity_id}</span></td>
                <td style="color:#334155;">${l.reasoning}</td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    `;
  },

  // ---------------- Operations ----------------
  async quickAdjustStock(skuId, delta, reason) {
    try {
      const res = await fetch('/api/skus/adjust', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sku_id: skuId, delta, reason })
      });
      if (res.ok) {
        this.showToast(`Stock adjusted by ${delta > 0 ? '+' : ''}${delta}`);
        await this.reloadEntity('skus');
        await this.reloadEntity('summary');
        await this.reloadEntity('auditLogs');
      } else {
        const err = await res.text();
        alert(err);
      }
    } catch (e) {
      console.error(e);
    }
  },

  async toggleShiftClock(shiftId, newStatus) {
    await fetch('/api/shifts/status', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ shift_id: shiftId, status: newStatus })
    });
    this.showToast(`Shift status updated to ${newStatus}`);
    await this.reloadEntity('shifts');
    await this.reloadEntity('auditLogs');
  },

  async cycleTaskStatus(taskId, currentStatus) {
    const next = currentStatus === 'todo' ? 'in_progress' : currentStatus === 'in_progress' ? 'done' : 'todo';
    await fetch('/api/tasks/status', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ task_id: taskId, status: next })
    });
    this.showToast(`Task status updated to ${next}`);
    await this.reloadEntity('tasks');
    await this.reloadEntity('auditLogs');
  },

  async syncAllNAP() {
    const first = this.data.listings[0] || {};
    await fetch('/api/listings/sync', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        business_name: first.nap_business_name || 'AOSphere Technologies Ltd',
        address: first.nap_address || '742 Innovation Blvd, Suite 400, Austin TX 78701',
        phone: first.nap_phone || '+1 (512) 555-0199',
        website: first.nap_website || 'https://aosphere.example.com'
      })
    });
    this.showToast('Synchronized NAP across Google, Yelp, Bing & Apple Maps');
    await this.reloadEntity('listings');
    await this.reloadEntity('auditLogs');
  },

  async saveAndSyncNAP() {
    const bName = document.getElementById('nap-name').value;
    const addr = document.getElementById('nap-address').value;
    const phone = document.getElementById('nap-phone').value;
    const web = document.getElementById('nap-website').value;

    await fetch('/api/listings/sync', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ business_name: bName, address: addr, phone, website: web })
    });
    this.showToast('Master business data saved & broadcasted');
    await this.reloadEntity('listings');
    await this.reloadEntity('auditLogs');
  },

  async publishReviewReply(reviewId) {
    const rev = this.data.reviews.find(r => r.id === reviewId);
    if (!rev) return;
    await fetch('/api/reviews/reply', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ review_id: reviewId, reply_text: rev.reply_text, publish: true })
    });
    this.showToast(`Reply published to ${rev.platform}`);
    await this.reloadEntity('reviews');
    await this.reloadEntity('auditLogs');
  },

  async draftAIReply(reviewId) {
    const rev = this.data.reviews.find(r => r.id === reviewId);
    if (!rev) return;
    const sampleReply = rev.rating >= 4
      ? `Thank you ${rev.author_name}! We are thrilled you had a seamless experience with our platform.`
      : `Hello ${rev.author_name}, we appreciate your honest feedback and have flagged this with our operations team for immediate follow-up.`;

    await fetch('/api/reviews/reply', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ review_id: reviewId, reply_text: sampleReply, publish: false })
    });
    this.showToast('AI agent drafted response. Ready for approval.');
    await this.reloadEntity('reviews');
    await this.reloadEntity('auditLogs');
  },

  async decideApproval(apprId, decision) {
    await fetch('/api/approvals/decide', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ approval_id: apprId, decision })
    });
    this.showToast(`Approval ${decision}`);
    await this.reloadEntity('approvals');
    await this.reloadEntity('skus');
    await this.reloadEntity('auditLogs');
  },

  // ---------------- Modals & Human Actions ----------------
  openStockModal(skuId, code, currentStock) {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:10px; color:var(--text-main);">Adjust Stock for ${code}</h3>
      <p style="font-size:13.5px; color:var(--text-muted); margin-bottom:16px;">
        Current stock: <strong>${currentStock}</strong>. If quantity drops below safety threshold, the system automatically triggers an automated purchase order!
      </p>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <label for="modal-stock-delta" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Quantity Delta (+/-)</label>
        <input type="number" id="modal-stock-delta" class="form-input" value="-10">
        <label for="modal-stock-reason" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Reason / Note</label>
        <input type="text" id="modal-stock-reason" class="form-input" value="Assembly line consumption">
        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitStockModal('${skuId}')">Submit Adjustment</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
  },

  async submitStockModal(skuId) {
    const delta = parseInt(document.getElementById('modal-stock-delta').value, 10);
    const reason = document.getElementById('modal-stock-reason').value;
    await this.quickAdjustStock(skuId, delta, reason);
    this.closeModal();
  },

  openNewSKUModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '560px';
    const supOptions = (this.data.suppliers || []).map(s => `<option value="${s.id}">${s.name}</option>`).join('') || '<option value="sup_01">Nexis Silicon</option>';

    content.innerHTML = `
      <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px;">
        <h3 style="font-size:18px; font-weight:800; color:var(--text-main);">Add New SKU</h3>
        <span class="actor-pill actor-human">HUMAN CRUD</span>
      </div>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="sku-code" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">SKU Code</label>
            <input type="text" id="sku-code" class="form-input code-input" placeholder="e.g. SKU-SEN-500">
          </div>
          <div>
            <label for="sku-category" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Category</label>
            <select id="sku-category" class="form-input">
              <option value="Hardware">Hardware</option>
              <option value="Electronics">Electronics</option>
              <option value="Sensors">Sensors</option>
              <option value="Accessories">Accessories</option>
              <option value="Software">Software</option>
            </select>
          </div>
        </div>
        <label for="sku-name" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Product Name</label>
        <input type="text" id="sku-name" class="form-input" placeholder="e.g. 4K Vision Sensor">
        <div style="display:grid; grid-template-columns:1fr 1fr 1fr; gap:12px;">
          <div>
            <label for="sku-stock" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Initial Stock</label>
            <input type="number" id="sku-stock" class="form-input" value="100">
          </div>
          <div>
            <label for="sku-min" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Min Threshold</label>
            <input type="number" id="sku-min" class="form-input" value="20">
          </div>
          <div>
            <label for="sku-reorder" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Reorder Qty</label>
            <input type="number" id="sku-reorder" class="form-input" value="50">
          </div>
        </div>
        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="sku-cost" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Unit Cost ($)</label>
            <input type="number" step="0.01" id="sku-cost" class="form-input" value="45.00">
          </div>
          <div>
            <label for="sku-price" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Selling Price ($)</label>
            <input type="number" step="0.01" id="sku-price" class="form-input" value="95.00">
          </div>
        </div>
        <label for="sku-supplier" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Primary Supplier</label>
        <select id="sku-supplier" class="form-input">
          ${supOptions}
        </select>
        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitNewSKU()">Create SKU</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
  },

  async submitNewSKU() {
    const code = document.getElementById('sku-code').value.trim();
    const name = document.getElementById('sku-name').value.trim();
    const category = document.getElementById('sku-category').value;
    const stock = parseInt(document.getElementById('sku-stock').value, 10);
    const min = parseInt(document.getElementById('sku-min').value, 10);
    const reorder = parseInt(document.getElementById('sku-reorder').value, 10);
    const cost = parseFloat(document.getElementById('sku-cost').value);
    const price = parseFloat(document.getElementById('sku-price').value);
    const supplier = document.getElementById('sku-supplier').value;

    if (!code || !name) {
      this.showToast('Please provide both SKU code and name');
      return;
    }

    try {
      const res = await fetch('/api/skus', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          code, name, category,
          stock_quantity: stock,
          min_threshold: min,
          reorder_quantity: reorder,
          unit_cost: cost,
          selling_price: price,
          supplier_id: supplier
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'Failed to create SKU'));
        return;
      }
      this.showToast(`SKU ${code} created successfully`);
      this.closeModal();
      await this.reloadEntity('skus');
      await this.reloadEntity('auditLogs');
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  openNewPOModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '540px';
    const supOptions = (this.data.suppliers || []).map(s => `<option value="${s.id}">${s.name} (${s.lead_time_days}d lead time)</option>`).join('');
    const skuOptions = (this.data.skus || []).map(k => `<option value="${k.code}" data-cost="${k.unit_cost}">${k.code} — ${k.name} ($${k.unit_cost.toFixed(2)})</option>`).join('');

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:10px; color:var(--text-main);">Create Purchase Order</h3>
      <p style="font-size:13.5px; color:var(--text-muted); margin-bottom:16px;">
        Issue supplier purchase order. Orders &gt; $1,000 automatically route to the Human-in-the-Loop Approval Inbox.
      </p>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <label for="po-supplier" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Target Supplier</label>
        <select id="po-supplier" class="form-input">
          ${supOptions}
        </select>
        <label for="po-sku" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Select SKU</label>
        <select id="po-sku" class="form-input" onchange="app.onPOSkuChange()">
          ${skuOptions}
        </select>
        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="po-qty" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Quantity</label>
            <input type="number" id="po-qty" class="form-input" value="50" oninput="app.calculatePOTotal()">
          </div>
          <div>
            <label for="po-cost" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Unit Cost ($)</label>
            <input type="number" id="po-cost" class="form-input" value="45" step="0.5" oninput="app.calculatePOTotal()">
          </div>
        </div>
        <div style="padding:14px; background:var(--bg-tertiary); border:1px solid var(--border-subtle); border-radius:12px; font-size:13px;">
          Estimated Total: <strong id="po-total-display" style="color:#059669; font-size:15px;">$2,250.00</strong>
          <span id="po-approval-hint" style="display:block; font-size:11.5px; color:#D97706; margin-top:4px; font-weight:600;">
            ⚠️ Total exceeds $1,000 safety threshold: will be queued for Human Approval.
          </span>
        </div>
        <label for="po-notes" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Notes / Instructions</label>
        <input type="text" id="po-notes" class="form-input" value="Stock replenishment for production batch">
        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitNewPO()">Create Purchase Order</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
    this.calculatePOTotal();
  },

  onPOSkuChange() {
    const sel = document.getElementById('po-sku');
    if (!sel) return;
    const opt = sel.options[sel.selectedIndex];
    if (opt && opt.dataset.cost) {
      document.getElementById('po-cost').value = parseFloat(opt.dataset.cost);
      this.calculatePOTotal();
    }
  },

  calculatePOTotal() {
    const qty = parseInt(document.getElementById('po-qty')?.value || 0, 10);
    const cost = parseFloat(document.getElementById('po-cost')?.value || 0);
    const total = qty * cost;
    const disp = document.getElementById('po-total-display');
    const hint = document.getElementById('po-approval-hint');
    if (disp) disp.textContent = `$${total.toLocaleString('en-US', { minimumFractionDigits: 2 })}`;
    if (hint) hint.style.display = total > 1000 ? 'block' : 'none';
  },

  async submitNewPO() {
    const supplierId = document.getElementById('po-supplier').value;
    const skuCode = document.getElementById('po-sku').value;
    const qty = parseInt(document.getElementById('po-qty').value, 10);
    const cost = parseFloat(document.getElementById('po-cost').value);
    const notes = document.getElementById('po-notes').value;
    const total = qty * cost;

    const items = JSON.stringify([{
      sku_code: skuCode,
      quantity: qty,
      unit_cost: cost
    }]);

    await fetch('/api/purchase-orders', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        supplier_id: supplierId,
        items_json: items,
        total: total,
        notes: notes
      })
    });

    this.showToast(total > 1000 ? 'PO created & sent to Approval Inbox' : 'Purchase Order drafted');
    this.closeModal();
    await this.reloadEntity('skus');
    await this.reloadEntity('approvals');
    await this.reloadEntity('auditLogs');
  },

  openNewDealModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '540px';

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:10px; color:var(--text-main);">Create New Deal</h3>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <label for="deal-title" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Deal Title</label>
        <input type="text" id="deal-title" class="form-input" placeholder="e.g. Enterprise Hardware License">
        <label for="deal-customer" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Customer</label>
        <select id="deal-customer" class="form-input">
          <option value="cust_01">Acme Corporation</option>
          <option value="cust_02">Starlight Systems</option>
          <option value="cust_03">Apex Logistics</option>
        </select>
        <label for="deal-value" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Deal Value ($)</label>
        <input type="number" id="deal-value" class="form-input" value="25000">
        <label for="deal-stage" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Stage</label>
        <select id="deal-stage" class="form-input">
          <option value="lead">New Leads</option>
          <option value="qualified">Qualified</option>
          <option value="proposal">Proposal</option>
          <option value="negotiation">Negotiation</option>
          <option value="won">Won</option>
        </select>
        <label for="deal-date" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Close Date</label>
        <input type="date" id="deal-date" class="form-input" value="2026-11-30">
        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitNewDeal()">Create Deal</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
  },

  async submitNewDeal() {
    const title = document.getElementById('deal-title').value;
    const customerId = document.getElementById('deal-customer')?.value || 'cust_01';
    const value = parseFloat(document.getElementById('deal-value').value);
    const stage = document.getElementById('deal-stage').value;
    const closeDate = document.getElementById('deal-date').value;

    await fetch('/api/deals', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title, customer_id: customerId, value, stage, expected_close_date: closeDate })
    });
    this.showToast('Deal created successfully');
    this.closeModal();
    await this.reloadEntity('deals');
    await this.reloadEntity('summary');
    await this.reloadEntity('auditLogs');
  },

  async convertDealToInvoice(dealId) {
    if (!confirm('Convert this sales deal into an official customer invoice?')) return;
    try {
      const res = await fetch('/api/deals/convert', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ deal_id: dealId })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'Failed to convert deal'));
        return;
      }
      const inv = await res.json();
      this.showToast(`Deal converted to invoice #${inv.invoice_number}`);
      await this.reloadEntity('deals');
      await this.reloadEntity('invoices');
      await this.reloadEntity('summary');
      await this.reloadEntity('auditLogs');
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  openNewInvoiceModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '680px';

    const custOptions = (this.data.deals || [])
      .filter(d => d.customer_name)
      .map(d => `<option value="${d.customer_id || 'cust_01'}">${d.customer_name} (${d.customer_company || 'Client'})</option>`)
      .filter((v, i, a) => a.indexOf(v) === i)
      .join('') || '<option value="cust_01">Acme Corporation</option><option value="cust_02">Starlight Systems</option>';

    content.innerHTML = `
      <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px;">
        <h3 style="font-size:18px; font-weight:800; color:var(--text-main);">Generate Multi-Line Invoice</h3>
        <span class="actor-pill actor-human">HUMAN AUTHOR</span>
      </div>
      <div style="display:flex; flex-direction:column; gap:14px;">
        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="inv-customer" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Customer / Client</label>
            <select id="inv-customer" class="form-input">
              ${custOptions}
            </select>
          </div>
          <div>
            <label for="inv-due" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Payment Due Date</label>
            <input type="date" id="inv-due" class="form-input" value="2026-11-20">
          </div>
        </div>

        <div>
          <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:6px;">
            <label style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Invoice Line Items</label>
            <button type="button" class="btn-press btn btn-secondary btn-sm" onclick="app.addInvoiceRow()" style="height:28px; font-size:11.5px;">
              <i data-lucide="plus" class="w-3 h-3"></i> Add Line
            </button>
          </div>
          <table class="data-table" style="font-size:13px;" id="inv-items-table">
            <thead>
              <tr>
                <th style="padding:8px 10px;">Item Description</th>
                <th style="padding:8px 10px; width:90px;">Qty</th>
                <th style="padding:8px 10px; width:110px;">Unit ($)</th>
                <th style="padding:8px 10px; width:110px;">Total ($)</th>
                <th style="padding:8px 10px; width:45px;"></th>
              </tr>
            </thead>
            <tbody id="inv-items-body">
              <tr class="inv-row">
                <td style="padding:6px 10px;">
                  <input type="text" class="form-input inv-item-desc" style="padding:6px 8px; font-size:12.5px;" value="Quantum Computing Node Deployment" list="sku-datalist">
                </td>
                <td style="padding:6px 10px;">
                  <input type="number" class="form-input inv-item-qty" style="padding:6px 8px; font-size:12.5px;" value="1" min="1" oninput="app.calculateInvoiceTotals()">
                </td>
                <td style="padding:6px 10px;">
                  <input type="number" class="form-input inv-item-price" style="padding:6px 8px; font-size:12.5px;" value="2400.00" step="0.5" oninput="app.calculateInvoiceTotals()">
                </td>
                <td style="padding:6px 10px; font-weight:700; color:var(--text-main);" class="inv-item-total">$2,400.00</td>
                <td style="padding:6px 10px; text-align:center;">
                  <button type="button" class="icon-btn" style="width:28px; height:28px;" onclick="app.removeInvoiceRow(this)" title="Remove item">
                    <i data-lucide="trash-2" class="w-3.5 h-3.5 text-rose-500"></i>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
          <datalist id="sku-datalist">
            ${(this.data.skus || []).map(k => `<option value="${k.name}">${k.code} - $${k.selling_price}</option>`).join('')}
          </datalist>
        </div>

        <!-- Totals summary box -->
        <div style="background:var(--bg-tertiary); border:1px solid var(--border-subtle); border-radius:12px; padding:12px 16px; display:flex; flex-direction:column; gap:6px;">
          <div style="display:flex; justify-content:space-between; font-size:13px; color:var(--text-muted);">
            <span>Subtotal:</span>
            <strong id="inv-subtotal" style="color:var(--text-main);">$2,400.00</strong>
          </div>
          <div style="display:flex; justify-content:space-between; font-size:13px; color:var(--text-muted);">
            <span>Tax (8%):</span>
            <strong id="inv-tax" style="color:var(--text-main);">$192.00</strong>
          </div>
          <div style="display:flex; justify-content:space-between; font-size:15px; font-weight:800; border-top:1px solid var(--border-subtle); padding-top:6px; color:var(--text-main);">
            <span>Grand Total:</span>
            <span id="inv-grand-total" style="color:#059669;">$2,592.00</span>
          </div>
        </div>

        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:8px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitNewInvoice()">
            <i data-lucide="file-check" class="w-4 h-4"></i> Issue Invoice
          </button>
        </div>
      </div>
    `;
    modal.classList.add('active');
    this.calculateInvoiceTotals();
    if (window.lucide) lucide.createIcons();
  },

  addInvoiceRow() {
    const tbody = document.getElementById('inv-items-body');
    if (!tbody) return;
    const tr = document.createElement('tr');
    tr.className = 'inv-row';
    tr.innerHTML = `
      <td style="padding:6px 10px;">
        <input type="text" class="form-input inv-item-desc" style="padding:6px 8px; font-size:12.5px;" placeholder="Item name or service..." list="sku-datalist">
      </td>
      <td style="padding:6px 10px;">
        <input type="number" class="form-input inv-item-qty" style="padding:6px 8px; font-size:12.5px;" value="1" min="1" oninput="app.calculateInvoiceTotals()">
      </td>
      <td style="padding:6px 10px;">
        <input type="number" class="form-input inv-item-price" style="padding:6px 8px; font-size:12.5px;" value="100.00" step="0.5" oninput="app.calculateInvoiceTotals()">
      </td>
      <td style="padding:6px 10px; font-weight:700; color:var(--text-main);" class="inv-item-total">$100.00</td>
      <td style="padding:6px 10px; text-align:center;">
        <button type="button" class="icon-btn" style="width:28px; height:28px;" onclick="app.removeInvoiceRow(this)" title="Remove item">
          <i data-lucide="trash-2" class="w-3.5 h-3.5 text-rose-500"></i>
        </button>
      </td>
    `;
    tbody.appendChild(tr);
    if (window.lucide) lucide.createIcons();
    this.calculateInvoiceTotals();
  },

  removeInvoiceRow(btn) {
    const row = btn.closest('tr');
    const tbody = document.getElementById('inv-items-body');
    if (tbody && tbody.querySelectorAll('tr').length > 1) {
      row.remove();
      this.calculateInvoiceTotals();
    } else {
      this.showToast('Invoice must have at least one line item');
    }
  },

  calculateInvoiceTotals() {
    const rows = document.querySelectorAll('.inv-row');
    let subtotal = 0;
    rows.forEach(r => {
      const qty = parseFloat(r.querySelector('.inv-item-qty')?.value || 0);
      const price = parseFloat(r.querySelector('.inv-item-price')?.value || 0);
      const rowTotal = qty * price;
      const totalCell = r.querySelector('.inv-item-total');
      if (totalCell) totalCell.textContent = `$${rowTotal.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
      subtotal += rowTotal;
    });

    const tax = subtotal * 0.08;
    const grand = subtotal + tax;

    const subEl = document.getElementById('inv-subtotal');
    const taxEl = document.getElementById('inv-tax');
    const grandEl = document.getElementById('inv-grand-total');

    if (subEl) subEl.textContent = `$${subtotal.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
    if (taxEl) taxEl.textContent = `$${tax.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
    if (grandEl) grandEl.textContent = `$${grand.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
  },

  async submitNewInvoice() {
    const custId = document.getElementById('inv-customer')?.value || 'cust_01';
    const dueDate = document.getElementById('inv-due')?.value || '2026-11-20';
    const rows = document.querySelectorAll('.inv-row');
    const items = [];
    let subtotal = 0;

    rows.forEach(r => {
      const desc = r.querySelector('.inv-item-desc')?.value.trim() || 'Service';
      const qty = parseFloat(r.querySelector('.inv-item-qty')?.value || 1);
      const price = parseFloat(r.querySelector('.inv-item-price')?.value || 0);
      items.push({ desc, qty, price });
      subtotal += qty * price;
    });

    if (items.length === 0) {
      this.showToast('Please add at least one line item');
      return;
    }

    const tax = subtotal * 0.08;

    try {
      const res = await fetch('/api/invoices', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          customer_id: custId,
          items_json: JSON.stringify(items),
          subtotal,
          tax,
          due_date: dueDate,
          recurring: 'none'
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'Failed to issue invoice'));
        return;
      }
      this.showToast(`Tax invoice generated for $${(subtotal + tax).toLocaleString('en-US', { minimumFractionDigits: 2 })}`);
      this.closeModal();
      await this.reloadEntity('invoices');
      await this.reloadEntity('summary');
      await this.reloadEntity('auditLogs');
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  closeModal(event) {
    if (event && event.target && !event.target.classList.contains('modal-backdrop')) return;
    const modal = document.getElementById('action-modal');
    if (modal) {
      modal.classList.remove('active');
      const win = modal.querySelector('.modal-window');
      if (win) win.style.maxWidth = '';
    }
  },

  // ---------------- MCP Drawer & Co-Pilot Inspector ----------------
  toggleAgentDrawer() {
    const drawer = document.getElementById('copilot-drawer');
    drawer.classList.toggle('open');
    if (window.lucide) lucide.createIcons();
  },

  setDrawerTab(tab) {
    this.drawerTab = tab;
    document.querySelectorAll('.drawer-tab').forEach((el, idx) => {
      const match = (idx === 0 && tab === 'stream') || (idx === 1 && tab === 'tools') || (idx === 2 && tab === 'tester');
      el.classList.toggle('active', match);
    });
    document.getElementById('drawer-stream-view').classList.toggle('active', tab === 'stream');
    document.getElementById('drawer-tools-view').classList.toggle('active', tab === 'tools');
    document.getElementById('drawer-tester-view').classList.toggle('active', tab === 'tester');
    if (window.lucide) lucide.createIcons();
  },

  renderDrawerTimeline() {
    const container = document.getElementById('agent-timeline');
    if (!container) return;

    const recent = this.data.auditLogs.slice(0, 10);
    container.innerHTML = recent.map(l => `
      <div class="timeline-event">
        <div class="event-header">
          <span class="actor-pill actor-${l.actor_type}">${l.actor_type.toUpperCase()}: ${l.actor_id}</span>
          <span class="event-time">${new Date(l.created_at).toLocaleTimeString()}</span>
        </div>
        <div style="font-weight:700; color:#0F172A; margin-bottom:2px;">${l.action}</div>
        <div style="color:var(--text-muted); font-size:12px;">${l.reasoning}</div>
      </div>
    `).join('');
  },

  async loadMCPTools() {
    const tools = [
      { name: 'erp_check_stock', desc: 'Inspect stock levels, reserved quantities, and threshold alerts for SKUs.' },
      { name: 'erp_adjust_stock', desc: 'Adjust SKU inventory quantity. If stock drops below threshold, automatically triggers reorder workflows.' },
      { name: 'erp_create_purchase_order', desc: 'Create a purchase order for a supplier. Orders > $1000 automatically route to the human approval inbox.' },
      { name: 'crm_manage_deal', desc: 'Update deal stage or log client notes.' },
      { name: 'fin_get_summary', desc: 'Get real-time Profit & Loss (P&L), unpaid client receivables, and financial health.' },
      { name: 'fin_create_invoice', desc: 'Generate a compliant client invoice with line items, tax, and optional recurring schedule.' },
      { name: 'fin_trial_balance', desc: 'Inspect the General Ledger trial balance, showing debits, credits, and balance verification across all accounts.' },
      { name: 'crm_convert_deal', desc: 'Converts an accepted sales deal into an official customer invoice with line items and financial tracking.' },
      { name: 'erp_pay_purchase_order', desc: 'Records vendor payment for a received Purchase Order and posts balancing entries to Accounts Payable and Cash ledger accounts.' },
      { name: 'hr_lock_payroll', desc: 'Locks all approved timesheets for a billing period, records internal payroll expense, and posts disbursements to General Ledger.' },
      { name: 'docs_search', desc: 'Searches documents and internal knowledge repository by keywords.' },
      { name: 'presence_sync_nap', desc: 'Synchronize official business Name, Address, Phone (NAP) across search engines and directories.' },
      { name: 'reputation_draft_reply', desc: 'Analyze review sentiment and draft an empathetic response for human review.' },
      { name: 'approvals_list_pending', desc: 'View actions waiting for human authorization in the Approval Inbox.' },
      { name: 'sys_reconcile', desc: 'Run business workflow reconciliation scan to detect orphan approvals and anomalies.' },
      { name: 'sys_backup', desc: 'Trigger an atomic online SQLite snapshot backup and run integrity check verification.' }
    ];

    this.data.tools = tools;

    const listContainer = document.getElementById('tools-list');
    if (listContainer) {
      listContainer.innerHTML = tools.map(t => `
        <div class="tool-card">
          <div class="tool-name">${t.name}</div>
          <div class="tool-desc">${t.desc}</div>
        </div>
      `).join('');
    }

    const select = document.getElementById('mcp-tool-select');
    if (select && select.options.length === 0) {
      select.innerHTML = tools.map(t => `<option value="${t.name}">${t.name}</option>`).join('');
      this.onToolSelectChange();
    }
  },

  onToolSelectChange() {
    const sel = document.getElementById('mcp-tool-select');
    const argsBox = document.getElementById('mcp-tool-args');
    if (!sel || !argsBox) return;

    const toolName = sel.value;
    const presets = {
      erp_check_stock: { below_threshold_only: true },
      erp_adjust_stock: { sku_id: 'sku_01', delta: -5, reason: 'Demo consumption' },
      erp_create_purchase_order: { supplier_id: 'sup_01', items: [{ sku_code: 'SKU-CORE-100', quantity: 20, unit_cost: 45.0 }] },
      erp_pay_purchase_order: { po_id: 'po_01' },
      crm_manage_deal: { deal_id: 'deal_01', stage: 'won' },
      crm_convert_deal: { deal_id: 'deal_01' },
      fin_get_summary: {},
      fin_create_invoice: { customer_id: 'cust_01', items: [{ desc: 'Hardware Support', qty: 1, price: 1200 }], due_date: '2026-11-20' },
      fin_trial_balance: {},
      hr_lock_payroll: { period: '2026-10' },
      docs_search: { query: 'sensor' },
      presence_sync_nap: { business_name: 'AOSphere Technologies Ltd', address: '742 Innovation Blvd, Suite 400', phone: '+1 (512) 555-0199', website: 'https://aosphere.example.com' },
      reputation_draft_reply: { review_id: 'rev_03', reply_text: 'Thank you for your feedback! We have resolved the shipping delay.' },
      approvals_list_pending: {},
      sys_reconcile: {},
      sys_backup: { prefix: 'manual_ui' }
    };

    argsBox.value = JSON.stringify(presets[toolName] || {}, null, 2);
  },

  async executeManualToolCall() {
    const sel = document.getElementById('mcp-tool-select');
    const argsBox = document.getElementById('mcp-tool-args');
    const outBox = document.getElementById('mcp-tool-output');
    const keyInput = document.getElementById('mcp-api-key');

    if (keyInput && keyInput.value) {
      this.apiKey = keyInput.value.trim();
      localStorage.setItem('aosphere_api_key', this.apiKey);
    }

    try {
      const toolName = sel.value;
      const args = JSON.parse(argsBox.value || '{}');

      outBox.textContent = 'Calling MCP tool via JSON-RPC 2.0...';

      const rpcReq = {
        jsonrpc: '2.0',
        id: Date.now(),
        method: 'tools/call',
        params: {
          name: toolName,
          arguments: args
        }
      };

      const res = await fetch('/mcp/messages', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${this.apiKey}`,
          'X-API-Key': this.apiKey
        },
        body: JSON.stringify(rpcReq)
      });

      const rpcResp = await res.json();
      outBox.textContent = JSON.stringify(rpcResp, null, 2);
      this.showToast(`MCP Tool ${toolName} executed`);
      await this.reloadEntity('skus');
      await this.reloadEntity('deals');
      await this.reloadEntity('invoices');
      await this.reloadEntity('approvals');
      await this.reloadEntity('summary');
      await this.reloadEntity('auditLogs');
    } catch (e) {
      outBox.textContent = 'Error: ' + e.message;
    }
  },

  // ---------------- Command Palette (Ctrl+K) ----------------
  openCommandPalette() {
    const modal = document.getElementById('command-palette-modal');
    modal.classList.add('active');
    const input = document.getElementById('palette-search-input');
    input.value = '';
    input.focus();
    this.filterCommands('');
    if (window.lucide) lucide.createIcons();
  },

  closeCommandPalette(e) {
    if (e && e.target && !e.target.classList.contains('modal-backdrop')) return;
    const modal = document.getElementById('command-palette-modal');
    modal.classList.remove('active');
  },

  filterCommands(query) {
    const results = document.getElementById('palette-results');
    const q = query.toLowerCase();

    const commands = [
      { title: 'Open ERP & Inventory Catalog', tag: 'Navigation', action: () => { app.setTab('erp'); app.closeCommandPalette(); } },
      { title: 'Open Visual Sales Pipeline (Kanban)', tag: 'Navigation', action: () => { app.setTab('crm'); app.closeCommandPalette(); } },
      { title: 'Open Financial Summary & Invoices', tag: 'Navigation', action: () => { app.setTab('finance'); app.closeCommandPalette(); } },
      { title: 'Open Shift Schedule & Team Records', tag: 'Navigation', action: () => { app.setTab('hr'); app.closeCommandPalette(); } },
      { title: 'Open Tasks & Delivery Board', tag: 'Navigation', action: () => { app.setTab('tasks'); app.closeCommandPalette(); } },
      { title: 'Open Local Listing Sync (NAP)', tag: 'Navigation', action: () => { app.setTab('presence'); app.closeCommandPalette(); } },
      { title: 'Open Reviews & Reputation Hub', tag: 'Navigation', action: () => { app.setTab('reviews'); app.closeCommandPalette(); } },
      { title: 'Open Approval Inbox', tag: 'Governance', action: () => { app.setTab('approvals'); app.closeCommandPalette(); } },
      { title: 'Open Dual-Actor Audit Trail', tag: 'Audit', action: () => { app.setTab('audit'); app.closeCommandPalette(); } },
      { title: 'Toggle MCP Agent Co-Pilot Drawer', tag: 'Agent', action: () => { app.toggleAgentDrawer(); app.closeCommandPalette(); } },
      { title: 'Trigger Immediate NAP Directory Sync', tag: 'Action', action: () => { app.syncAllNAP(); app.closeCommandPalette(); } },
      { title: 'Create Client Invoice', tag: 'Action', action: () => { app.closeCommandPalette(); app.openNewInvoiceModal(); } },
      { title: 'Create Supplier Purchase Order', tag: 'Action', action: () => { app.closeCommandPalette(); app.openNewPOModal(); } },
      { title: 'Add New Sales Deal', tag: 'Action', action: () => { app.closeCommandPalette(); app.openNewDealModal(); } }
    ];

    const filtered = commands.filter(c => c.title.toLowerCase().includes(q) || c.tag.toLowerCase().includes(q));

    results.innerHTML = filtered.map((c, idx) => `
      <div class="palette-item" onclick="app.runPaletteAction(${idx})">
        <span>${c.title}</span>
        <span class="palette-tag">${c.tag}</span>
      </div>
    `).join('');

    this._currentFilteredCommands = filtered;
  },

  runPaletteAction(idx) {
    if (this._currentFilteredCommands && this._currentFilteredCommands[idx]) {
      this._currentFilteredCommands[idx].action();
    }
  },

  // ---------------- Toast Notifications ----------------
  showToast(msg) {
    const container = document.getElementById('toast-container');
    if (!container) return;
    const toast = document.createElement('div');
    toast.className = 'toast';
    toast.innerHTML = `<i data-lucide="check-circle" class="w-4 h-4 text-emerald-400"></i> ${msg}`;
    container.appendChild(toast);
    if (window.lucide) lucide.createIcons();
    setTimeout(() => {
      toast.style.opacity = '0';
      toast.style.transform = 'translateY(10px)';
      toast.style.transition = 'all 0.3s ease';
      setTimeout(() => toast.remove(), 300);
    }, 3200);
  }
};

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
  app.init();
});
