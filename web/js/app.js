// ==========================================================================
// AO Sphere — Unified Operating System for Modern Business
// Client Application Engine (Minimal & Modern Theme)
// ==========================================================================

const app = {
  activeTab: 'dashboard',
  activeERPSubTab: 'skus',
  activeFinanceSubTab: 'invoices',
  activeCurrency: localStorage.getItem('aosphere_currency') || 'INR',
  drawerTab: 'stream',
  theme: 'light',
  apiKey: localStorage.getItem('aosphere_api_key') || 'ao_sk_agent_default_token_12345',
  data: {
    skus: [],
    suppliers: [],
    purchaseOrders: [],
    warehouses: [],
    warehouseStocks: [],
    grns: [],
    supplierBills: [],
    stockTransfers: [],
    batchLots: [],
    deals: [],
    invoices: [],
    creditNotes: [],
    accounts: [],
    journalEntries: [],
    currencies: [],
    exchangeRates: {
      INR: 83.50, USD: 1.0, EUR: 0.92, GBP: 0.78, JPY: 155.0, CNY: 7.23, SGD: 1.35,
      AED: 3.6725, SAR: 3.75, QAR: 3.64, KWD: 0.31, BHD: 0.376, OMR: 0.385, HKD: 7.82,
      KRW: 1370.0, TWD: 32.4, THB: 36.8, IDR: 16200.0, MYR: 4.71, PHP: 58.2, VND: 25400.0,
      BDT: 117.5, PKR: 278.5, LKR: 302.0, NPR: 133.6, BTN: 83.5, MVR: 15.4, AFN: 71.5,
      KHR: 4100.0, LAK: 21800.0, MMK: 2100.0, BND: 1.35, KZT: 448.0, UZS: 12650.0,
      CAD: 1.37, AUD: 1.52
    },
    currencySymbols: {
      INR: '₹', USD: '$', EUR: '€', GBP: '£', JPY: '¥', CNY: '¥', SGD: 'S$',
      AED: 'د.إ', SAR: '﷼', QAR: 'QR', KWD: 'KD', BHD: 'BD', OMR: 'OMR', HKD: 'HK$',
      KRW: '₩', TWD: 'NT$', THB: '฿', IDR: 'Rp', MYR: 'RM', PHP: '₱', VND: '₫',
      BDT: '৳', PKR: '₨', LKR: 'Rs', NPR: 'Rs', BTN: 'Nu.', MVR: 'Rf', AFN: '؋',
      KHR: '៛', LAK: '₭', MMK: 'K', BND: 'B$', KZT: '₸', UZS: 'UZS', CAD: 'C$', AUD: 'A$'
    },
    zeroDecimalCurrencies: ['JPY', 'KRW', 'VND', 'IDR', 'KHR', 'LAK', 'MMK', 'UZS'],
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
    this.setupCurrency();
    this.setupShortcuts();
    this.setupSSE();
    this.loadAllData();
    if (window.lucide) {
      lucide.createIcons();
    }
  },

  setupCurrency() {
    const saved = localStorage.getItem('aosphere_currency') || 'INR';
    this.activeCurrency = saved;
    const picker = document.getElementById('global-currency-picker');
    if (picker) picker.value = saved;
  },

  setCurrency(cur) {
    this.activeCurrency = cur;
    localStorage.setItem('aosphere_currency', cur);
    const picker = document.getElementById('global-currency-picker');
    if (picker) picker.value = cur;
    this.showToast(`Valuation currency set to ${cur}`);
    this.render();
  },

  setERPSubTab(subTab) {
    this.activeERPSubTab = subTab;
    this.render();
    if (window.lucide) lucide.createIcons();
  },

  setFinanceSubTab(subTab) {
    this.activeFinanceSubTab = subTab;
    this.render();
    if (window.lucide) lucide.createIcons();
  },

  formatMoney(amountInUSD, targetCur = null) {
    const cur = targetCur || this.activeCurrency || 'INR';
    const rate = (this.data.exchangeRates && this.data.exchangeRates[cur]) ? this.data.exchangeRates[cur] : 1.0;
    const symbol = (this.data.currencySymbols && this.data.currencySymbols[cur]) ? this.data.currencySymbols[cur] : cur + ' ';
    const isZeroDecimal = (this.data.zeroDecimalCurrencies && this.data.zeroDecimalCurrencies.includes(cur));

    const converted = (parseFloat(amountInUSD) || 0) * rate;

    if (cur === 'INR') {
      const isNegative = converted < 0;
      const absVal = Math.abs(converted);
      const [intPart, decPart] = absVal.toFixed(2).split('.');
      const lastThree = intPart.slice(-3);
      const otherNumbers = intPart.slice(0, -3);
      const formatted = otherNumbers !== '' ? otherNumbers.replace(/\B(?=(\d{2})+(?!\d))/g, ",") + "," + lastThree : lastThree;
      return `${isNegative ? '-' : ''}${symbol}${formatted}.${decPart}`;
    }

    if (isZeroDecimal) {
      const rounded = Math.round(converted);
      return `${symbol}${rounded.toLocaleString('en-US')}`;
    }

    return `${symbol}${converted.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
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
        stock_updated: ['skus', 'summary', 'warehouseStocks'],
        sku_created: ['skus'],
        po_created: ['skus', 'purchaseOrders', 'approvals', 'summary'],
        po_received: ['skus', 'purchaseOrders', 'summary', 'warehouseStocks'],
        po_paid: ['purchaseOrders', 'summary'],
        deal_created: ['deals', 'summary'],
        deal_stage_changed: ['deals', 'summary'],
        deal_converted: ['deals', 'invoices', 'summary'],
        quote_generated: ['deals'],
        invoice_created: ['invoices', 'summary'],
        shift_updated: ['shifts'],
        shift_created: ['shifts'],
        task_updated: ['tasks'],
        task_created: ['tasks'],
        employee_created: ['employees'],
        grn_received: ['grns', 'warehouseStocks', 'skus', 'purchaseOrders', 'summary'],
        stock_transferred: ['warehouseStocks', 'stockTransfers', 'skus'],
        supplier_bill_created: ['supplierBills', 'summary'],
        three_way_matched: ['supplierBills', 'approvals', 'summary'],
        nap_synced: ['listings'],
        review_replied: ['reviews'],
        approval_decided: ['approvals', 'skus', 'purchaseOrders', 'supplierBills'],
        approval_required: ['approvals'],
        credit_note_issued: ['creditNotes', 'accounts', 'journalEntries', 'invoices', 'summary'],
        exchange_rate_updated: ['currencies']
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
      purchaseOrders: '/api/purchase-orders',
      warehouses: '/api/warehouses',
      warehouseStocks: '/api/warehouse-stocks',
      grns: '/api/grns',
      supplierBills: '/api/supplier-bills',
      stockTransfers: '/api/stock-transfers',
      batchLots: '/api/batch-lots',
      deals: '/api/deals',
      invoices: '/api/invoices',
      creditNotes: '/api/credit-notes',
      accounts: '/api/ledger/accounts',
      journalEntries: '/api/ledger/entries',
      currencies: '/api/currencies',
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
        const payload = await res.json();
        if (name === 'currencies' && payload.rates) {
          this.data.exchangeRates = Object.assign({}, this.data.exchangeRates, payload.rates);
          this.data.currencies = payload.currencies || [];
        } else {
          this.data[name] = payload;
        }
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
      const [
        skus,
        suppliers,
        purchaseOrders,
        warehouses,
        warehouseStocks,
        grns,
        supplierBills,
        stockTransfers,
        batchLots,
        deals,
        invoices,
        creditNotes,
        accounts,
        journalEntries,
        currenciesResp,
        summary,
        employees,
        shifts,
        tasks,
        listings,
        reviews,
        approvals,
        auditLogs
      ] = await Promise.all([
        fetch('/api/skus').then(r => r.json()).catch(() => []),
        fetch('/api/suppliers').then(r => r.json()).catch(() => []),
        fetch('/api/purchase-orders').then(r => r.json()).catch(() => []),
        fetch('/api/warehouses').then(r => r.json()).catch(() => []),
        fetch('/api/warehouse-stocks').then(r => r.json()).catch(() => []),
        fetch('/api/grns').then(r => r.json()).catch(() => []),
        fetch('/api/supplier-bills').then(r => r.json()).catch(() => []),
        fetch('/api/stock-transfers').then(r => r.json()).catch(() => []),
        fetch('/api/batch-lots').then(r => r.json()).catch(() => []),
        fetch('/api/deals').then(r => r.json()).catch(() => []),
        fetch('/api/invoices').then(r => r.json()).catch(() => []),
        fetch('/api/credit-notes').then(r => r.json()).catch(() => []),
        fetch('/api/ledger/accounts').then(r => r.json()).catch(() => []),
        fetch('/api/ledger/entries').then(r => r.json()).catch(() => []),
        fetch('/api/currencies').then(r => r.json()).catch(() => null),
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
      this.data.purchaseOrders = purchaseOrders;
      this.data.warehouses = warehouses;
      this.data.warehouseStocks = warehouseStocks;
      this.data.grns = grns;
      this.data.supplierBills = supplierBills;
      this.data.stockTransfers = stockTransfers;
      this.data.batchLots = batchLots;
      this.data.deals = deals;
      this.data.invoices = invoices;
      this.data.creditNotes = creditNotes;
      this.data.accounts = accounts;
      this.data.journalEntries = journalEntries;
      if (currenciesResp && currenciesResp.rates) {
        this.data.exchangeRates = Object.assign({}, this.data.exchangeRates, currenciesResp.rates);
        this.data.currencies = currenciesResp.currencies || [];
      }
      this.data.summary = summary;
      this.data.employees = employees;
      this.data.shifts = shifts;
      this.data.tasks = tasks;
      this.data.listings = listings;
      this.data.reviews = reviews;
      this.data.approvals = approvals;
      this.data.auditLogs = auditLogs;

      const picker = document.getElementById('global-currency-picker');
      if (picker) picker.value = this.activeCurrency;

      this.updateBadges();
      this.render();
      this.renderDrawerTimeline();
      this.loadMCPTools();
    } catch (err) {
      console.error('Error loading platform data:', err);
    }
  },

  // ---------------- Sub-Tab & Filter Utilities ----------------
  setERPSubTab(subTab) {
    this.activeERPSubTab = subTab;
    this.render();
  },

  setFinanceSubTab(subTab) {
    this.activeFinanceSubTab = subTab;
    this.render();
  },

  filterTable(tableId, query) {
    const q = (query || '').toLowerCase().trim();
    const table = document.getElementById(tableId);
    if (!table) return;
    const tbody = table.querySelector('tbody');
    if (!tbody) return;
    const rows = tbody.querySelectorAll('tr');
    rows.forEach(r => {
      const text = r.textContent.toLowerCase();
      r.style.display = text.includes(q) ? '' : 'none';
    });
  },

  filterCards(containerId, cardClass, query) {
    const q = (query || '').toLowerCase().trim();
    const container = document.getElementById(containerId);
    if (!container) return;
    const cards = container.querySelectorAll('.' + cardClass);
    cards.forEach(c => {
      const text = c.textContent.toLowerCase();
      c.style.display = text.includes(q) ? '' : 'none';
    });
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
            <div class="stat-title">Cash Revenue (${this.activeCurrency})</div>
            <div class="w-10 h-10 bg-blue-50 rounded-xl flex items-center justify-center">
              <i data-lucide="trending-up" class="w-5 h-5 text-brand-blue"></i>
            </div>
          </div>
          <div class="stat-value">${this.formatMoney(s.total_revenue)}</div>
          <div class="stat-trend trend-up">
            <i data-lucide="check" class="w-3.5 h-3.5 inline"></i> Paid Invoices (${s.paid_count || 0})
          </div>
        </div>

        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-3">
            <div class="stat-title">Net Profit (P&L) (${this.activeCurrency})</div>
            <div class="w-10 h-10 bg-emerald-50 rounded-xl flex items-center justify-center">
              <i data-lucide="shopping-cart" class="w-5 h-5 text-emerald-500"></i>
            </div>
          </div>
          <div class="stat-value" style="color: #059669;">${this.formatMoney(s.net_profit)}</div>
          <div class="stat-trend trend-up">
            Expenses: ${this.formatMoney(s.total_expense)}
          </div>
        </div>

        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-3">
            <div class="stat-title">Active Deals Pipeline (${this.activeCurrency})</div>
            <div class="w-10 h-10 bg-purple-50 rounded-xl flex items-center justify-center">
              <i data-lucide="users" class="w-5 h-5 text-purple-500"></i>
            </div>
          </div>
          <div class="stat-value">${this.formatMoney(pipelineTotal)}</div>
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

  // ---------------- Module 2: ERP & Multi-Warehouse Supply Chain ----------------
  renderERP() {
    const subTab = this.activeERPSubTab || 'skus';

    let actionButtons = '';
    if (subTab === 'skus') {
      actionButtons = `
        <button class="btn-press btn btn-secondary" onclick="app.openNewSKUModal()">
          <i data-lucide="package-plus" class="w-4 h-4 text-brand-blue"></i> Add SKU
        </button>
        <button class="btn-press btn btn-primary" onclick="app.openNewPOModal()">
          <i data-lucide="plus" class="w-4 h-4"></i> New Purchase Order
        </button>
      `;
    } else if (subTab === 'pos') {
      actionButtons = `
        <button class="btn-press btn btn-primary" onclick="app.openNewPOModal()">
          <i data-lucide="plus" class="w-4 h-4"></i> New Purchase Order
        </button>
      `;
    } else if (subTab === 'warehouses') {
      actionButtons = `
        <button class="btn-press btn btn-secondary" onclick="app.openNewWarehouseModal()">
          <i data-lucide="building" class="w-4 h-4 text-brand-blue"></i> Add Warehouse
        </button>
        <button class="btn-press btn btn-primary" onclick="app.openStockTransferModal()">
          <i data-lucide="arrow-left-right" class="w-4 h-4"></i> Transfer Stock
        </button>
      `;
    } else if (subTab === 'grns') {
      actionButtons = `
        <button class="btn-press btn btn-primary" onclick="app.openReceiveGRNModal()">
          <i data-lucide="package-check" class="w-4 h-4"></i> Receive Goods (GRN)
        </button>
      `;
    }

    let subTabContent = '';
    if (subTab === 'skus') {
      subTabContent = `
        <div class="table-container mockup-item">
          <div class="table-filter-bar">
            <input type="text" class="table-filter-input" placeholder="Search SKUs by name, code, category..." oninput="app.filterTable('skus-table', this.value)">
            <span class="text-[12px] text-slate-400 font-medium">Catalog items: <strong>${this.data.skus.length}</strong></span>
          </div>
          <table class="data-table" id="skus-table">
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
    } else if (subTab === 'pos') {
      subTabContent = `
        <div class="table-container mockup-item">
          <div class="table-filter-bar">
            <input type="text" class="table-filter-input" placeholder="Search purchase orders by supplier, status..." oninput="app.filterTable('pos-table', this.value)">
            <span class="text-[12px] text-slate-400 font-medium">Orders: <strong>${this.data.purchaseOrders.length}</strong></span>
          </div>
          <table class="data-table" id="pos-table">
            <thead>
              <tr>
                <th>PO ID</th>
                <th>Supplier</th>
                <th>Total Value</th>
                <th>Status</th>
                <th>Notes</th>
                <th>Created</th>
                <th>Receipt Actions</th>
              </tr>
            </thead>
            <tbody>
              ${this.data.purchaseOrders.length === 0 ? `<tr><td colspan="7" class="text-center py-6 text-slate-400">No purchase orders created yet. Click "+ New Purchase Order" to issue one.</td></tr>` :
                this.data.purchaseOrders.map(po => {
                  const sup = (this.data.suppliers || []).find(s => s.id === po.supplier_id) || { name: 'Direct Supplier' };
                  const isReceivable = po.status === 'ordered' || po.status === 'partially_received' || po.status === 'approved';
                  return `
                    <tr>
                      <td><span class="mono-code">${po.id}</span></td>
                      <td><strong>${sup.name}</strong></td>
                      <td><strong style="color:#059669; font-size:14px;">${this.formatMoney(po.total_amount ?? po.total ?? 0)}</strong></td>
                      <td>
                        <span class="badge" style="background:${
                          po.status === 'received' ? '#ECFDF5' :
                          po.status === 'partially_received' ? '#EFF6FF' :
                          po.status === 'ordered' ? '#FEF3C7' :
                          po.status === 'approved' ? '#F0FDF4' : '#F1F5F9'
                        }; color:${
                          po.status === 'received' ? '#047857' :
                          po.status === 'partially_received' ? '#1D4ED8' :
                          po.status === 'ordered' ? '#B45309' :
                          po.status === 'approved' ? '#15803D' : '#475569'
                        };">
                          ${po.status.replace('_', ' ').toUpperCase()}
                        </span>
                      </td>
                      <td class="text-slate-500 font-medium">${po.notes || '—'}</td>
                      <td class="text-slate-400 font-mono text-[12px]">${new Date(po.created_at).toLocaleDateString()}</td>
                      <td>
                        <div style="display:flex; align-items:center; gap:6px;">
                          <a href="/api/purchase-orders/pdf?id=${po.id}&currency=${app.activeCurrency}" target="_blank" class="btn-pdf" title="Download Vector Purchase Order PDF">
                            <i data-lucide="file-down" class="w-3.5 h-3.5"></i> PDF
                          </a>
                          ${isReceivable ? `
                            <button class="btn-press btn btn-primary btn-sm" onclick="app.openReceiveGRNModal('${po.id}')">
                              <i data-lucide="package-check" class="w-3.5 h-3.5"></i> GRN
                            </button>
                          ` : po.status === 'received' ? `
                            <span class="badge badge-matched"><i data-lucide="check" class="w-3 h-3 inline"></i> Received</span>
                          ` : `
                            <span class="badge badge-pending">${po.status}</span>
                          `}
                        </div>
                      </td>
                    </tr>
                  `;
                }).join('')}
            </tbody>
          </table>
        </div>
      `;
    } else if (subTab === 'warehouses') {
      subTabContent = `
        <!-- Warehouse Cards Grid -->
        <div class="stats-grid mb-6">
          ${this.data.warehouses.map(wh => {
            const stocksInWH = (this.data.warehouseStocks || []).filter(ws => ws.warehouse_id === wh.id);
            const totalUnits = stocksInWH.reduce((sum, s) => sum + s.available_qty, 0);
            return `
              <div class="stat-card mockup-item">
                <div class="flex justify-between items-start mb-2">
                  <div>
                    <span class="mono-code text-[11px] font-bold text-brand-blue">${wh.code}</span>
                    <h3 class="font-extrabold text-[16px] text-slate-800 mt-0.5">${wh.name}</h3>
                  </div>
                  <div class="w-9 h-9 rounded-xl bg-blue-50 text-brand-blue flex items-center justify-center">
                    <i data-lucide="building" class="w-4 h-4"></i>
                  </div>
                </div>
                <div class="text-[12.5px] text-slate-500 mb-3">${wh.location} • <span class="text-slate-400">${wh.description || 'Fulfillment center'}</span></div>
                <div class="flex items-center justify-between pt-3 border-t border-slate-100">
                  <span class="text-[12px] font-medium text-slate-500">Holdings: <strong>${stocksInWH.length} SKUs</strong></span>
                  <span class="text-[13px] font-bold text-emerald-600">${totalUnits.toLocaleString()} units</span>
                </div>
              </div>
            `;
          }).join('')}
        </div>

        <!-- Multi-Warehouse Stock Breakdown -->
        <div class="table-container mockup-item mb-6">
          <div class="table-filter-bar">
            <input type="text" class="table-filter-input" placeholder="Search stock by warehouse, SKU, bin..." oninput="app.filterTable('wh-stocks-table', this.value)">
            <span class="text-[12px] text-slate-400 font-medium">Warehouse stock allocations: <strong>${this.data.warehouseStocks.length}</strong></span>
          </div>
          <table class="data-table" id="wh-stocks-table">
            <thead>
              <tr>
                <th>Warehouse</th>
                <th>SKU Code</th>
                <th>Bin Location</th>
                <th>Available Units</th>
                <th>Reserved Units</th>
                <th>Last Updated</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              ${this.data.warehouseStocks.length === 0 ? `<tr><td colspan="7" class="text-center py-6 text-slate-400">No warehouse stock allocations yet. Stock will appear when GRNs or transfers occur.</td></tr>` :
                this.data.warehouseStocks.map(ws => {
                  const wh = (this.data.warehouses || []).find(w => w.id === ws.warehouse_id) || { name: ws.warehouse_id, code: '' };
                  return `
                    <tr>
                      <td><strong>${wh.name}</strong> <span class="mono-code text-[11px]">${wh.code}</span></td>
                      <td><span class="mono-code font-bold text-brand-blue">${ws.sku_code}</span></td>
                      <td><span class="badge" style="background:#F1F5F9; color:#475569;">${ws.bin_location || 'A-01-01'}</span></td>
                      <td><strong style="font-size:14px; color:${ws.available_qty > 0 ? '#059669' : '#DC2626'};">${ws.available_qty}</strong></td>
                      <td class="text-slate-500 font-medium">${ws.reserved_qty}</td>
                      <td class="text-slate-400 font-mono text-[12px]">${new Date(ws.updated_at).toLocaleDateString()}</td>
                      <td>
                        <button class="btn-press btn btn-secondary btn-sm" onclick="app.openStockTransferModal('${ws.warehouse_id}', '${ws.sku_code}')">
                          Transfer
                        </button>
                      </td>
                    </tr>
                  `;
                }).join('')}
            </tbody>
          </table>
        </div>

        <!-- Stock Transfers History -->
        <div class="table-container mockup-item">
          <div style="padding:16px 20px; font-weight:800; font-size:15px; border-bottom:1px solid #F1F5F9; display:flex; justify-content:space-between; align-items:center;">
            <span>Inter-Warehouse Stock Transfers</span>
            <span class="text-[12px] text-slate-400 font-medium">Recorded: ${this.data.stockTransfers.length}</span>
          </div>
          <table class="data-table" id="wh-transfers-table">
            <thead>
              <tr>
                <th>Transfer #</th>
                <th>Source Warehouse</th>
                <th>Destination Warehouse</th>
                <th>SKU Code</th>
                <th>Quantity</th>
                <th>Status</th>
                <th>Reason</th>
                <th>Date</th>
              </tr>
            </thead>
            <tbody>
              ${this.data.stockTransfers.length === 0 ? `<tr><td colspan="8" class="text-center py-6 text-slate-400">No stock transfers executed yet. Click "Transfer Stock" above to move inventory between hubs.</td></tr>` :
                this.data.stockTransfers.map(tr => {
                  const srcWH = (this.data.warehouses || []).find(w => w.id === tr.from_warehouse_id) || { name: tr.from_warehouse_id };
                  const dstWH = (this.data.warehouses || []).find(w => w.id === tr.to_warehouse_id) || { name: tr.to_warehouse_id };
                  return `
                    <tr>
                      <td><span class="mono-code">${tr.transfer_number}</span></td>
                      <td>${srcWH.name}</td>
                      <td>${dstWH.name}</td>
                      <td><span class="mono-code font-bold text-brand-blue">${tr.sku_code}</span></td>
                      <td><strong style="color:#2563EB;">${tr.quantity}</strong></td>
                      <td><span class="badge badge-matched">${tr.status.toUpperCase()}</span></td>
                      <td class="text-slate-500 font-medium">${tr.reason || 'Balancing'}</td>
                      <td class="text-slate-400 font-mono text-[12px]">${new Date(tr.created_at).toLocaleDateString()}</td>
                    </tr>
                  `;
                }).join('')}
            </tbody>
          </table>
        </div>
      `;
    } else if (subTab === 'grns') {
      subTabContent = `
        <div class="table-container mockup-item">
          <div class="table-filter-bar">
            <input type="text" class="table-filter-input" placeholder="Search GRN #, PO #, inspector..." oninput="app.filterTable('grns-table', this.value)">
            <span class="text-[12px] text-slate-400 font-medium">Receipt Notes: <strong>${this.data.grns.length}</strong></span>
          </div>
          <table class="data-table" id="grns-table">
            <thead>
              <tr>
                <th>GRN Number</th>
                <th>Purchase Order</th>
                <th>Destination Warehouse</th>
                <th>Received Date</th>
                <th>Inspector / Agent</th>
                <th>Tracking / Carrier</th>
                <th>Status</th>
                <th>Notes</th>
              </tr>
            </thead>
            <tbody>
              ${this.data.grns.length === 0 ? `<tr><td colspan="8" class="text-center py-6 text-slate-400">No Goods Receipt Notes recorded yet. Receive items from approved purchase orders.</td></tr>` :
                this.data.grns.map(g => {
                  const wh = (this.data.warehouses || []).find(w => w.id === g.warehouse_id) || { name: g.warehouse_id };
                  return `
                    <tr>
                      <td><span class="mono-code font-bold text-brand-blue">${g.grn_number}</span></td>
                      <td><span class="mono-code">${g.po_id}</span></td>
                      <td><strong>${wh.name}</strong></td>
                      <td class="text-slate-500">${g.received_date}</td>
                      <td class="font-medium text-slate-700">${g.inspector_name || 'Autonomous Agent'}</td>
                      <td class="text-slate-500 font-mono text-[12px]">${g.carrier_tracking || '—'}</td>
                      <td><span class="badge badge-matched">${g.status.toUpperCase()}</span></td>
                      <td class="text-slate-500 font-medium">${g.notes || '—'}</td>
                    </tr>
                  `;
                }).join('')}
            </tbody>
          </table>
        </div>
      `;
    }

    return `
      <div class="page-header">
        <div>
          <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-blue-100 text-brand-blue text-[11px] font-bold uppercase tracking-widest mb-3">
            <i data-lucide="network" class="w-3.5 h-3.5"></i> Core Infrastructure & Supply Chain
          </div>
          <h1 class="page-title">Enterprise Resource Planning</h1>
          <p class="page-sub">Centralize multi-warehouse supply chain, physical Goods Receipt Notes (GRN), and reorder workflows.</p>
        </div>
        <div class="page-actions">
          ${actionButtons}
        </div>
      </div>

      <!-- Subnav navigation bar -->
      <div class="subnav-bar">
        <button class="subnav-pill ${subTab === 'skus' ? 'active' : ''}" onclick="app.setERPSubTab('skus')">
          <i data-lucide="package" class="w-4 h-4"></i> SKUs & Catalog (${this.data.skus.length})
        </button>
        <button class="subnav-pill ${subTab === 'pos' ? 'active' : ''}" onclick="app.setERPSubTab('pos')">
          <i data-lucide="file-text" class="w-4 h-4"></i> Purchase Orders (${this.data.purchaseOrders.length})
        </button>
        <button class="subnav-pill ${subTab === 'warehouses' ? 'active' : ''}" onclick="app.setERPSubTab('warehouses')">
          <i data-lucide="building" class="w-4 h-4"></i> Warehouses & Multi-Site (${this.data.warehouses.length})
        </button>
        <button class="subnav-pill ${subTab === 'grns' ? 'active' : ''}" onclick="app.setERPSubTab('grns')">
          <i data-lucide="package-check" class="w-4 h-4"></i> Goods Receipt Notes (${this.data.grns.length})
        </button>
      </div>

      ${subTabContent}
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

  // ---------------- Module 4: Finance, General Ledger & Multi-Currency ----------------
  renderFinance() {
    const s = this.data.summary || { total_revenue: 0, total_expense: 0, net_profit: 0, unpaid_total: 0 };
    const subTab = this.activeFinanceSubTab || 'invoices';

    let actionButton = '';
    if (subTab === 'invoices') {
      actionButton = `
        <button class="btn-press btn btn-primary" onclick="app.openNewInvoiceModal()">
          <i data-lucide="plus" class="w-4 h-4"></i> Create Invoice
        </button>
      `;
    } else if (subTab === 'bills') {
      actionButton = `
        <button class="btn-press btn btn-primary" onclick="app.openNewSupplierBillModal()">
          <i data-lucide="plus" class="w-4 h-4"></i> Add Supplier Bill
        </button>
      `;
    } else if (subTab === 'credit_notes') {
      actionButton = `
        <button class="btn-press btn btn-primary" onclick="app.openNewCreditNoteModal()">
          <i data-lucide="plus" class="w-4 h-4"></i> Issue Credit Note
        </button>
      `;
    } else if (subTab === 'ledger') {
      actionButton = `
        <button class="btn-press btn btn-secondary" onclick="app.reloadEntity('accounts'); app.reloadEntity('journalEntries'); app.showToast('Trial Balance & Ledgers synced');">
          <i data-lucide="refresh-cw" class="w-4 h-4"></i> Refresh Ledger
        </button>
      `;
    }

    let subTabContent = '';
    if (subTab === 'invoices') {
      subTabContent = `
        <div class="stats-grid">
          <div class="stat-card mockup-item">
            <div class="flex justify-between items-start mb-2">
              <div class="stat-title">Cash Revenue (${this.activeCurrency})</div>
              <div class="w-10 h-10 bg-blue-50 rounded-xl flex items-center justify-center">
                <i data-lucide="trending-up" class="w-5 h-5 text-brand-blue"></i>
              </div>
            </div>
            <div class="stat-value">${this.formatMoney(s.total_revenue)}</div>
            <div class="stat-trend trend-up">✓ Collected</div>
          </div>

          <div class="stat-card mockup-item">
            <div class="flex justify-between items-start mb-2">
              <div class="stat-title">Operating Expenses (${this.activeCurrency})</div>
              <div class="w-10 h-10 bg-purple-50 rounded-xl flex items-center justify-center">
                <i data-lucide="pie-chart" class="w-5 h-5 text-purple-600"></i>
              </div>
            </div>
            <div class="stat-value">${this.formatMoney(s.total_expense)}</div>
            <div class="stat-trend text-slate-500 font-medium">Procurement & Operations</div>
          </div>

          <div class="stat-card mockup-item">
            <div class="flex justify-between items-start mb-2">
              <div class="stat-title">Net Profit (P&L)</div>
              <div class="w-10 h-10 bg-emerald-50 rounded-xl flex items-center justify-center">
                <i data-lucide="check" class="w-5 h-5 text-emerald-500"></i>
              </div>
            </div>
            <div class="stat-value" style="color:#059669;">${this.formatMoney(s.net_profit)}</div>
            <div class="stat-trend trend-up">P&L Margin: Healthy</div>
          </div>

          <div class="stat-card mockup-item">
            <div class="flex justify-between items-start mb-2">
              <div class="stat-title">Pending Receivables (${this.activeCurrency})</div>
              <div class="w-10 h-10 bg-amber-50 rounded-xl flex items-center justify-center">
                <i data-lucide="clock" class="w-5 h-5 text-amber-500"></i>
              </div>
            </div>
            <div class="stat-value" style="color:#D97706;">${this.formatMoney(s.unpaid_total)}</div>
            <div class="stat-trend trend-amber">Uncollected Due</div>
          </div>
        </div>

        <div class="table-container mockup-item">
          <div class="table-filter-bar">
            <input type="text" class="table-filter-input" placeholder="Search invoices by customer, number..." oninput="app.filterTable('invoices-table', this.value)">
            <span class="text-[12px] text-slate-400 font-medium">Invoices: <strong>${this.data.invoices.length}</strong></span>
          </div>
          <table class="data-table" id="invoices-table">
            <thead>
              <tr>
                <th>Invoice #</th>
                <th>Customer</th>
                <th>Issue Date</th>
                <th>Due Date</th>
                <th>Subtotal</th>
                <th>Tax</th>
                <th>Total (${this.activeCurrency})</th>
                <th>Status</th>
                <th>Official Document</th>
              </tr>
            </thead>
            <tbody>
              ${this.data.invoices.map(inv => `
                <tr>
                  <td><span class="mono-code">${inv.invoice_number}</span></td>
                  <td><strong>${inv.customer_name || 'Client'}</strong></td>
                  <td class="text-slate-500">${inv.issue_date}</td>
                  <td class="text-slate-500">${inv.due_date}</td>
                  <td class="font-medium">${this.formatMoney(inv.subtotal)}</td>
                  <td class="text-slate-400">${this.formatMoney(inv.tax_amount)}</td>
                  <td><strong style="color:#059669; font-size:14px;">${this.formatMoney(inv.total_amount)}</strong></td>
                  <td>
                    <span class="badge" style="background:${inv.status === 'paid' ? '#ECFDF5' : '#FEF3C7'}; color:${inv.status === 'paid' ? '#047857' : '#B45309'}; border:1px solid ${inv.status === 'paid' ? '#A7F3D0' : '#FDE68A'};">
                      ${inv.status.toUpperCase()}
                    </span>
                  </td>
                  <td>
                    <a href="/api/invoices/pdf?id=${inv.id}&currency=${app.activeCurrency}" target="_blank" class="btn-pdf" title="Download Vector PDF Invoice">
                      <i data-lucide="file-down" class="w-3.5 h-3.5"></i> PDF
                    </a>
                  </td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>
      `;
    } else if (subTab === 'bills') {
      subTabContent = `
        <!-- 3-Way Match Explanatory Banner -->
        <div style="background:#EFF6FF; border:1px solid #BFDBFE; border-radius:16px; padding:18px 22px; margin-bottom:20px; display:flex; align-items:flex-start; gap:16px;">
          <div class="w-10 h-10 rounded-xl bg-blue-600 text-white flex items-center justify-center shrink-0 shadow-sm">
            <i data-lucide="scale" class="w-5 h-5"></i>
          </div>
          <div>
            <h3 class="font-bold text-[15px] text-blue-950 mb-1">Automated 3-Way Reconciliation Engine</h3>
            <p class="text-[13px] text-blue-800 leading-relaxed">
              Every vendor bill is verified across three independent sources: the authorized <strong>Purchase Order contract price</strong>, the warehouse's physical <strong>Goods Receipt Note (GRN) received count</strong>, and the <strong>Supplier Bill total</strong>. Price discrepancies (&gt; $50) or quantity shortfalls route directly to the <strong>Human-in-the-Loop Approval Inbox</strong>.
            </p>
          </div>
        </div>

        <div class="table-container mockup-item">
          <div class="table-filter-bar">
            <input type="text" class="table-filter-input" placeholder="Search bills by bill #, supplier, PO..." oninput="app.filterTable('bills-table', this.value)">
            <span class="text-[12px] text-slate-400 font-medium">Supplier Bills: <strong>${this.data.supplierBills.length}</strong></span>
          </div>
          <table class="data-table" id="bills-table">
            <thead>
              <tr>
                <th>Bill Number</th>
                <th>Supplier</th>
                <th>Purchase Order</th>
                <th>GRN Reference</th>
                <th>Billed Total (${this.activeCurrency})</th>
                <th>Due Date</th>
                <th>3-Way Match Status</th>
                <th>Verification Action</th>
              </tr>
            </thead>
            <tbody>
              ${this.data.supplierBills.length === 0 ? `<tr><td colspan="8" class="text-center py-6 text-slate-400">No supplier bills recorded yet. Click "+ Add Supplier Bill" to enter a vendor invoice.</td></tr>` :
                this.data.supplierBills.map(b => {
                  const sup = (this.data.suppliers || []).find(s => s.id === b.supplier_id) || { name: 'Supplier' };
                  let badge = '';
                  if (b.match_status === 'matched') {
                    badge = `<span class="badge badge-matched"><i data-lucide="check-circle" class="w-3.5 h-3.5 inline"></i> MATCHED</span>`;
                  } else if (b.match_status === 'price_variance') {
                    badge = `<span class="badge badge-variance-price"><i data-lucide="alert-triangle" class="w-3.5 h-3.5 inline"></i> PRICE VARIANCE</span>`;
                  } else if (b.match_status === 'qty_variance') {
                    badge = `<span class="badge badge-variance-qty"><i data-lucide="alert-circle" class="w-3.5 h-3.5 inline"></i> QTY SHORTFALL</span>`;
                  } else {
                    badge = `<span class="badge badge-pending">PENDING AUDIT</span>`;
                  }
                  return `
                    <tr>
                      <td><span class="mono-code font-bold text-brand-blue">${b.bill_number}</span></td>
                      <td><strong>${sup.name}</strong></td>
                      <td><span class="mono-code">${b.po_id || '—'}</span></td>
                      <td><span class="mono-code">${b.grn_id || '—'}</span></td>
                      <td><strong style="color:#0F172A; font-size:14px;">${this.formatMoney(b.amount)}</strong></td>
                      <td class="text-slate-500">${b.due_date}</td>
                      <td>${badge}</td>
                      <td>
                        <button class="btn-press btn btn-secondary btn-sm" onclick="app.runThreeWayMatch('${b.id}')">
                          <i data-lucide="scan" class="w-3.5 h-3.5 text-blue-600"></i> Verify Match
                        </button>
                      </td>
                    </tr>
                  `;
                }).join('')}
            </tbody>
          </table>
        </div>
      `;
    } else if (subTab === 'credit_notes') {
      subTabContent = `
        <div style="background:#EFF6FF; border:1px solid #BFDBFE; border-radius:16px; padding:18px 22px; margin-bottom:20px; display:flex; align-items:flex-start; gap:16px;">
          <div class="w-10 h-10 rounded-xl bg-blue-600 text-white flex items-center justify-center shrink-0 shadow-sm">
            <i data-lucide="file-minus" class="w-5 h-5"></i>
          </div>
          <div>
            <h3 class="font-bold text-[15px] text-blue-950 mb-1">Credit Notes & Double-Entry Reversal Engine</h3>
            <p class="text-[13px] text-blue-800 leading-relaxed">
              Issuing a credit note automatically records an immutable general ledger reversal: <strong>Debiting Sales Revenue (Account 4000)</strong> and <strong>Sales Tax Payable (Account 2100)</strong> while <strong>Crediting Accounts Receivable (Account 1100)</strong>. Vector PDF documents can be generated and emailed to customers instantly.
            </p>
          </div>
        </div>

        <div class="table-container mockup-item">
          <div class="table-filter-bar">
            <input type="text" class="table-filter-input" placeholder="Search credit notes by #, customer, invoice..." oninput="app.filterTable('credit-notes-table', this.value)">
            <span class="text-[12px] text-slate-400 font-medium">Credit Notes Issued: <strong>${(this.data.creditNotes || []).length}</strong></span>
          </div>
          <table class="data-table" id="credit-notes-table">
            <thead>
              <tr>
                <th>Credit Note #</th>
                <th>Original Invoice</th>
                <th>Customer</th>
                <th>Reason</th>
                <th>Subtotal</th>
                <th>Tax</th>
                <th>Total Reversal</th>
                <th>Status</th>
                <th>Date</th>
                <th>Official Document</th>
              </tr>
            </thead>
            <tbody>
              ${(this.data.creditNotes || []).length === 0 ? `<tr><td colspan="10" class="text-center py-6 text-slate-400">No credit notes issued yet. Click "+ Issue Credit Note" to issue a billing reversal.</td></tr>` :
                this.data.creditNotes.map(cn => `
                  <tr>
                    <td><span class="mono-code font-bold text-rose-600">${cn.credit_note_number}</span></td>
                    <td><span class="mono-code">${cn.invoice_number || cn.invoice_id || '—'}</span></td>
                    <td><strong>${cn.customer_name || 'Customer'}</strong></td>
                    <td><span class="badge badge-credit-note">${(cn.reason || 'billing_error').replace('_', ' ').toUpperCase()}</span></td>
                    <td class="font-medium text-slate-600">${this.formatMoney(cn.subtotal, cn.currency || this.activeCurrency)}</td>
                    <td class="text-slate-400">${this.formatMoney(cn.tax_amount, cn.currency || this.activeCurrency)}</td>
                    <td><strong class="text-rose-600 font-bold" style="font-size:14px;">-${this.formatMoney(cn.total_amount, cn.currency || this.activeCurrency)}</strong></td>
                    <td><span class="badge badge-credit-issued">${(cn.status || 'ISSUED').toUpperCase()}</span></td>
                    <td class="text-slate-500 font-mono text-[12px]">${new Date(cn.created_at).toLocaleDateString()}</td>
                    <td>
                      <a href="/api/credit-notes/pdf?id=${cn.id}&currency=${cn.currency || this.activeCurrency}" target="_blank" class="btn-pdf" title="Download Vector Credit Note PDF">
                        <i data-lucide="file-down" class="w-3.5 h-3.5"></i> PDF
                      </a>
                    </td>
                  </tr>
                `).join('')}
            </tbody>
          </table>
        </div>
      `;
    } else if (subTab === 'ledger') {
      const accounts = this.data.accounts || [];
      const entries = this.data.journalEntries || [];
      const totalDebits = accounts.reduce((sum, a) => sum + (a.debit_cents || 0), 0) / 100.0;
      const totalCredits = accounts.reduce((sum, a) => sum + (a.credit_cents || 0), 0) / 100.0;
      const isBalanced = Math.abs(totalDebits - totalCredits) < 0.01;

      subTabContent = `
        <!-- Trial Balance Invariant Status Banner -->
        <div class="trial-balance-banner">
          <div style="display:flex; align-items:center; gap:14px;">
            <div class="w-10 h-10 rounded-xl ${isBalanced ? 'bg-emerald-600 text-white' : 'bg-rose-600 text-white'} flex items-center justify-center shrink-0 shadow-sm">
              <i data-lucide="${isBalanced ? 'check-check' : 'alert-circle'}" class="w-5 h-5"></i>
            </div>
            <div>
              <h3 class="font-bold text-[15px] text-slate-800 mb-0.5">Trial Balance Invariant Status: ${isBalanced ? '<span class="text-emerald-600">PERFECTLY BALANCED</span>' : '<span class="text-rose-600">UNBALANCED</span>'}</h3>
              <p class="text-[12.5px] text-slate-500">
                Mathematical accounting identity: <code>Debits = Credits</code>. Total Debits: <strong>${this.formatMoney(totalDebits)}</strong> • Total Credits: <strong>${this.formatMoney(totalCredits)}</strong> • Net Difference: <strong>$0.00</strong>
              </p>
            </div>
          </div>
          <div>
            <span class="badge ${isBalanced ? 'badge-matched' : 'badge-variance-price'}" style="font-size:12px; padding:6px 12px;">
              ${isBalanced ? '✓ DOUBLE-ENTRY BALANCED' : '⚠️ RECONCILIATION NEEDED'}
            </span>
          </div>
        </div>

        <!-- Chart of Accounts Grid -->
        <div class="table-container mockup-item mb-6">
          <div class="table-filter-bar">
            <span class="font-bold text-[14px] text-slate-800">Chart of Accounts (${accounts.length} Ledgers)</span>
            <input type="text" class="table-filter-input" placeholder="Search account by code, name, type..." oninput="app.filterTable('coa-table', this.value)">
          </div>
          <table class="data-table" id="coa-table">
            <thead>
              <tr>
                <th>Code</th>
                <th>Account Name</th>
                <th>Classification</th>
                <th>Normal Balance</th>
                <th>Total Debits</th>
                <th>Total Credits</th>
                <th>Net Balance (${this.activeCurrency})</th>
              </tr>
            </thead>
            <tbody>
              ${accounts.length === 0 ? `<tr><td colspan="7" class="text-center py-6 text-slate-400">Loading Chart of Accounts...</td></tr>` :
                accounts.map(acc => {
                  const deb = (acc.debit_cents || 0) / 100.0;
                  const cred = (acc.credit_cents || 0) / 100.0;
                  const net = acc.net_balance !== undefined ? acc.net_balance : (acc.normal_balance === 'debit' ? (deb - cred) : (cred - deb));
                  return `
                    <tr>
                      <td><span class="mono-code font-bold text-brand-blue">${acc.code}</span></td>
                      <td><strong>${acc.name}</strong></td>
                      <td><span class="badge" style="background:#F1F5F9; color:#475569;">${acc.type.toUpperCase()}</span></td>
                      <td><span class="mono-code text-[11px] uppercase">${acc.normal_balance}</span></td>
                      <td class="font-mono text-[13px] text-blue-600">${this.formatMoney(deb)}</td>
                      <td class="font-mono text-[13px] text-emerald-600">${this.formatMoney(cred)}</td>
                      <td><strong class="font-mono text-[13.5px] ${net >= 0 ? 'text-slate-800' : 'text-rose-600'}">${this.formatMoney(net)}</strong></td>
                    </tr>
                  `;
                }).join('')}
            </tbody>
          </table>
        </div>

        <!-- Live Posted Journal Entries Stream -->
        <div class="table-container mockup-item">
          <div class="table-filter-bar">
            <span class="font-bold text-[14px] text-slate-800">Live Posted Journal Entries (${entries.length} Entries)</span>
            <span class="text-[12px] text-slate-400">Strict Double-Entry Audit Trail</span>
          </div>
          <table class="data-table" id="journal-entries-table">
            <thead>
              <tr>
                <th>Entry #</th>
                <th>Date</th>
                <th>Source</th>
                <th>Memo / Description</th>
                <th>Debits</th>
                <th>Credits</th>
                <th>Posted By</th>
              </tr>
            </thead>
            <tbody>
              ${entries.length === 0 ? `<tr><td colspan="7" class="text-center py-6 text-slate-400">No journal entries posted yet. Transactions from Invoices and Credit Notes will appear here automatically.</td></tr>` :
                entries.map(je => {
                  const lines = je.lines || [];
                  const debitLines = lines.filter(l => l.entry_type === 'debit');
                  const creditLines = lines.filter(l => l.entry_type === 'credit');
                  return `
                    <tr>
                      <td><span class="mono-code font-bold text-brand-blue">${je.entry_number}</span></td>
                      <td class="text-slate-500 font-mono text-[12px]">${je.date || new Date(je.posted_at).toLocaleDateString()}</td>
                      <td><span class="badge" style="background:#F8FAFC; color:#64748B;">${(je.source || 'manual').toUpperCase()}</span></td>
                      <td class="font-medium text-slate-700">${je.memo || 'Transaction'}</td>
                      <td>
                        ${debitLines.map(dl => `
                          <div style="margin-bottom:3px;">
                            <span class="debit-tag">Dr</span> <span class="mono-code text-[11px]">${dl.account_code || ''}</span> <strong>${this.formatMoney((dl.amount_cents || 0)/100.0)}</strong>
                          </div>
                        `).join('')}
                      </td>
                      <td>
                        ${creditLines.map(cl => `
                          <div style="margin-bottom:3px;">
                            <span class="credit-tag">Cr</span> <span class="mono-code text-[11px]">${cl.account_code || ''}</span> <strong>${this.formatMoney((cl.amount_cents || 0)/100.0)}</strong>
                          </div>
                        `).join('')}
                      </td>
                      <td class="text-slate-400 font-mono text-[11.5px]">${je.posted_by || 'system'}</td>
                    </tr>
                  `;
                }).join('')}
            </tbody>
          </table>
        </div>
      `;
    }

    return `
      <div class="page-header">
        <div>
          <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-purple-100 text-purple-700 text-[11px] font-bold uppercase tracking-widest mb-3">
            <i data-lucide="pie-chart" class="w-3.5 h-3.5"></i> Stability & Capital
          </div>
          <h1 class="page-title">Financial & Administrative</h1>
          <p class="page-sub">General Ledger accounting, multi-currency reporting (${this.activeCurrency}), credit notes, and 3-Way Matching.</p>
        </div>
        <div class="page-actions">
          ${actionButton}
        </div>
      </div>

      <div class="subnav-bar">
        <button class="subnav-pill ${subTab === 'invoices' ? 'active' : ''}" onclick="app.setFinanceSubTab('invoices')">
          <i data-lucide="receipt" class="w-4 h-4"></i> Customer Invoices (${this.data.invoices.length})
        </button>
        <button class="subnav-pill ${subTab === 'bills' ? 'active' : ''}" onclick="app.setFinanceSubTab('bills')">
          <i data-lucide="scale" class="w-4 h-4"></i> Supplier Bills & 3-Way Match (${this.data.supplierBills.length})
        </button>
        <button class="subnav-pill ${subTab === 'credit_notes' ? 'active' : ''}" onclick="app.setFinanceSubTab('credit_notes')">
          <i data-lucide="file-minus" class="w-4 h-4"></i> Credit Notes & Reversals (${(this.data.creditNotes || []).length})
        </button>
        <button class="subnav-pill ${subTab === 'ledger' ? 'active' : ''}" onclick="app.setFinanceSubTab('ledger')">
          <i data-lucide="book-open" class="w-4 h-4"></i> General Ledger & COA (${(this.data.accounts || []).length})
        </button>
      </div>

      ${subTabContent}
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
        <div class="page-actions">
          <button class="btn-press btn btn-secondary" onclick="app.openNewEmployeeModal()">
            <i data-lucide="user-plus" class="w-4 h-4 text-brand-blue"></i> Add Employee
          </button>
          <button class="btn-press btn btn-primary" onclick="app.openNewShiftModal()">
            <i data-lucide="calendar-plus" class="w-4 h-4"></i> Schedule Shift
          </button>
        </div>
      </div>

      <div style="display:grid; grid-template-columns: 1.8fr 1.2fr; gap:24px;">
        
        <!-- Shifts Schedule Grid -->
        <div class="table-container mockup-item">
          <div class="table-filter-bar">
            <span class="font-bold text-[15px] text-slate-800">Today's Shift Roster</span>
            <input type="text" class="table-filter-input" placeholder="Search shifts by employee, station..." oninput="app.filterTable('shifts-table', this.value)">
          </div>
          <table class="data-table" id="shifts-table">
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
              ${this.data.shifts.length === 0 ? `<tr><td colspan="5" class="text-center py-6 text-slate-400">No shifts scheduled for today. Click "+ Schedule Shift" to assign a shift.</td></tr>` :
                this.data.shifts.map(sh => `
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
          <div class="flex justify-between items-center mb-4">
            <h3 style="font-size:16px; font-weight:800; color:#0F172A;">Active Employees (${this.data.employees.length})</h3>
            <input type="text" class="table-filter-input" style="max-width:180px; height:32px; font-size:11.5px;" placeholder="Filter team..." oninput="app.filterCards('employees-list-container', 'emp-item', this.value)">
          </div>
          <div id="employees-list-container" style="display:flex; flex-direction:column; gap:12px; max-height:480px; overflow-y:auto;">
            ${this.data.employees.length === 0 ? `<div class="text-slate-400 text-sm py-4">No employees registered. Click "+ Add Employee" above.</div>` :
              this.data.employees.map(emp => `
                <div class="emp-item" style="display:flex; align-items:center; justify-content:space-between; padding:12px 14px; background:#F8FAFC; border:1px solid var(--border-subtle); border-radius:12px;">
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
        <div class="page-actions">
          <button class="btn-press btn btn-primary" onclick="app.openNewTaskModal()">
            <i data-lucide="plus" class="w-4 h-4"></i> New Task
          </button>
        </div>
      </div>

      <div class="table-container mockup-item">
        <div class="table-filter-bar">
          <input type="text" class="table-filter-input" placeholder="Search tasks by title, project, assignee..." oninput="app.filterTable('tasks-table', this.value)">
          <span class="text-[12px] text-slate-400 font-medium">Total tasks: <strong>${this.data.tasks.length}</strong></span>
        </div>
        <table class="data-table" id="tasks-table">
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
            ${this.data.tasks.length === 0 ? `<tr><td colspan="7" class="text-center py-6 text-slate-400">No tasks created yet. Click "+ New Task" to add one.</td></tr>` :
              this.data.tasks.map(t => `
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
        <div style="display:grid; grid-template-columns:1fr 1fr 1fr; gap:12px;">
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
          <div>
            <label for="inv-recurring" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Recurring Frequency</label>
            <select id="inv-recurring" class="form-input">
              <option value="none">None (One-time)</option>
              <option value="weekly">Weekly</option>
              <option value="monthly">Monthly</option>
              <option value="annually">Annually</option>
            </select>
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
    const recurring = document.getElementById('inv-recurring')?.value || 'none';
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
          recurring
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
      this.showToast('Failed to issue invoice: ' + e.message);
    }
  },

  // ---------------- HR & Team Modals ----------------
  openNewEmployeeModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '500px';

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:10px; color:var(--text-main);">Add New Team Member</h3>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <label for="emp-name" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Full Name</label>
        <input type="text" id="emp-name" class="form-input" placeholder="e.g. Jessica Chen">
        
        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="emp-role" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Role</label>
            <input type="text" id="emp-role" class="form-input" value="Inventory Specialist">
          </div>
          <div>
            <label for="emp-dept" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Department</label>
            <input type="text" id="emp-dept" class="form-input" value="Operations">
          </div>
        </div>

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="emp-rate" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Hourly Rate ($)</label>
            <input type="number" id="emp-rate" class="form-input" value="38.00" step="1">
          </div>
          <div>
            <label for="emp-email" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Email Address</label>
            <input type="email" id="emp-email" class="form-input" placeholder="jessica@example.com">
          </div>
        </div>

        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitNewEmployee()">Create Employee</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
  },

  async submitNewEmployee() {
    const name = document.getElementById('emp-name')?.value.trim();
    const role = document.getElementById('emp-role')?.value.trim();
    const dept = document.getElementById('emp-dept')?.value.trim();
    const rate = parseFloat(document.getElementById('emp-rate')?.value || 0);
    const email = document.getElementById('emp-email')?.value.trim();

    if (!name || !role) {
      this.showToast('Please enter both name and role');
      return;
    }

    try {
      const res = await fetch('/api/employees', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, role, department: dept, hourly_rate: rate, email })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'Failed to add employee'));
        return;
      }
      this.showToast(`Employee ${name} added successfully`);
      this.closeModal();
      await this.reloadEntity('employees');
      await this.reloadEntity('auditLogs');
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  openNewShiftModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '500px';

    const empOptions = (this.data.employees || []).map(e => `<option value="${e.id}">${e.name} (${e.role})</option>`).join('') || '<option value="emp_01">Sarah Jenkins</option>';

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:10px; color:var(--text-main);">Schedule Shift</h3>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <label for="shift-emp" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Employee</label>
        <select id="shift-emp" class="form-input">
          ${empOptions}
        </select>
        
        <label for="shift-date" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Shift Date</label>
        <input type="date" id="shift-date" class="form-input" value="2026-10-26">

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="shift-start" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Start Time</label>
            <input type="text" id="shift-start" class="form-input" value="09:00 AM">
          </div>
          <div>
            <label for="shift-end" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">End Time</label>
            <input type="text" id="shift-end" class="form-input" value="05:30 PM">
          </div>
        </div>

        <label for="shift-station" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Workstation / Area</label>
        <input type="text" id="shift-station" class="form-input" value="Logistics Bay A">

        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitNewShift()">Save Shift</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
  },

  async submitNewShift() {
    const empId = document.getElementById('shift-emp')?.value;
    const shiftDate = document.getElementById('shift-date')?.value;
    const start = document.getElementById('shift-start')?.value.trim();
    const end = document.getElementById('shift-end')?.value.trim();
    const station = document.getElementById('shift-station')?.value.trim();

    try {
      const res = await fetch('/api/shifts', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          employee_id: empId,
          shift_date: shiftDate,
          start_time: start,
          end_time: end,
          station: station
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'Failed to schedule shift'));
        return;
      }
      this.showToast('Shift scheduled successfully');
      this.closeModal();
      await this.reloadEntity('shifts');
      await this.reloadEntity('auditLogs');
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  openNewTaskModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '520px';

    const empOptions = (this.data.employees || []).map(e => `<option value="${e.id}">${e.name} (${e.role})</option>`).join('') || '<option value="emp_01">Sarah Jenkins</option>';

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:10px; color:var(--text-main);">Create New Task</h3>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <label for="task-title" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Task Title</label>
        <input type="text" id="task-title" class="form-input" placeholder="e.g. Conduct Warehouse Bin Audit">

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="task-project" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Project</label>
            <input type="text" id="task-project" class="form-input" value="Supply Chain Scale">
          </div>
          <div>
            <label for="task-assignee" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Assignee</label>
            <select id="task-assignee" class="form-input">
              ${empOptions}
            </select>
          </div>
        </div>

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="task-priority" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Priority</label>
            <select id="task-priority" class="form-input">
              <option value="medium">Medium</option>
              <option value="high">High</option>
              <option value="urgent">Urgent</option>
              <option value="low">Low</option>
            </select>
          </div>
          <div>
            <label for="task-due" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Due Date</label>
            <input type="date" id="task-due" class="form-input" value="2026-11-05">
          </div>
        </div>

        <label for="task-desc" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Description</label>
        <textarea id="task-desc" class="form-input" rows="3">Verify physical inventory counts against system records across storage bins.</textarea>

        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitNewTask()">Create Task</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
  },

  async submitNewTask() {
    const title = document.getElementById('task-title')?.value.trim();
    const project = document.getElementById('task-project')?.value.trim();
    const assigneeId = document.getElementById('task-assignee')?.value;
    const priority = document.getElementById('task-priority')?.value;
    const due = document.getElementById('task-due')?.value;
    const desc = document.getElementById('task-desc')?.value.trim();

    if (!title) {
      this.showToast('Please enter a task title');
      return;
    }

    try {
      const res = await fetch('/api/tasks', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          title,
          project_name: project,
          assignee_id: assigneeId,
          priority,
          due_date: due,
          description: desc
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'Failed to create task'));
        return;
      }
      this.showToast(`Task "${title}" created`);
      this.closeModal();
      await this.reloadEntity('tasks');
      await this.reloadEntity('auditLogs');
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  // ---------------- Supply Chain & Multi-Warehouse Modals ----------------
  openNewWarehouseModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '500px';

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:10px; color:var(--text-main);">Add New Warehouse</h3>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div style="display:grid; grid-template-columns:1fr 1.5fr; gap:12px;">
          <div>
            <label for="wh-code" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Warehouse Code</label>
            <input type="text" id="wh-code" class="form-input" placeholder="e.g. WH-SOUTH">
          </div>
          <div>
            <label for="wh-name" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Facility Name</label>
            <input type="text" id="wh-name" class="form-input" placeholder="e.g. South Distribution Hub">
          </div>
        </div>

        <label for="wh-location" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Location / Address</label>
        <input type="text" id="wh-location" class="form-input" placeholder="e.g. 500 Airport Way, Dallas, TX">

        <label for="wh-desc" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Description / Specialization</label>
        <input type="text" id="wh-desc" class="form-input" value="Bulk storage & secondary fulfillment hub">

        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitNewWarehouse()">Create Warehouse</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
  },

  async submitNewWarehouse() {
    const code = document.getElementById('wh-code')?.value.trim();
    const name = document.getElementById('wh-name')?.value.trim();
    const location = document.getElementById('wh-location')?.value.trim();
    const desc = document.getElementById('wh-desc')?.value.trim();

    if (!code || !name) {
      this.showToast('Please provide both warehouse code and name');
      return;
    }

    try {
      const res = await fetch('/api/warehouses', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code, name, location, description: desc })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'Failed to add warehouse'));
        return;
      }
      this.showToast(`Warehouse ${name} (${code}) added`);
      this.closeModal();
      await this.reloadEntity('warehouses');
      await this.reloadEntity('auditLogs');
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  openStockTransferModal(sourceWhId = '', skuCode = '') {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '540px';

    const whOptionsFrom = (this.data.warehouses || []).map(w => `<option value="${w.id}" ${w.id === sourceWhId ? 'selected' : ''}>${w.name} (${w.code})</option>`).join('');
    const whOptionsTo = (this.data.warehouses || []).map(w => `<option value="${w.id}" ${w.id !== sourceWhId ? 'selected' : ''}>${w.name} (${w.code})</option>`).join('');
    const skuOptions = (this.data.skus || []).map(s => `<option value="${s.code}" ${s.code === skuCode ? 'selected' : ''}>${s.code} — ${s.name}</option>`).join('');

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:10px; color:var(--text-main);">Transfer Stock Between Warehouses</h3>
      <p style="font-size:13px; color:var(--text-muted); margin-bottom:14px;">
        Execute an atomic inter-warehouse inventory movement with optimistic balance verification.
      </p>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="tr-source" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Source Warehouse</label>
            <select id="tr-source" class="form-input">
              ${whOptionsFrom}
            </select>
          </div>
          <div>
            <label for="tr-dest" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Destination Warehouse</label>
            <select id="tr-dest" class="form-input">
              ${whOptionsTo}
            </select>
          </div>
        </div>

        <div style="display:grid; grid-template-columns:1.5fr 1fr; gap:12px;">
          <div>
            <label for="tr-sku" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Select SKU</label>
            <select id="tr-sku" class="form-input">
              ${skuOptions}
            </select>
          </div>
          <div>
            <label for="tr-qty" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Quantity</label>
            <input type="number" id="tr-qty" class="form-input" value="10" min="1">
          </div>
        </div>

        <label for="tr-reason" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Transfer Reason / Memo</label>
        <input type="text" id="tr-reason" class="form-input" value="Regional rebalancing & order fulfillment prep">

        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitStockTransfer()">
            <i data-lucide="arrow-left-right" class="w-4 h-4"></i> Execute Transfer
          </button>
        </div>
      </div>
    `;
    modal.classList.add('active');
  },

  async submitStockTransfer() {
    const fromWh = document.getElementById('tr-source')?.value;
    const toWh = document.getElementById('tr-dest')?.value;
    const skuCode = document.getElementById('tr-sku')?.value;
    const qty = parseInt(document.getElementById('tr-qty')?.value || 0, 10);
    const reason = document.getElementById('tr-reason')?.value.trim();

    if (fromWh === toWh) {
      this.showToast('Source and destination warehouses cannot be the same');
      return;
    }
    if (qty <= 0) {
      this.showToast('Transfer quantity must be greater than 0');
      return;
    }

    try {
      const res = await fetch('/api/stock-transfers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          from_warehouse_id: fromWh,
          to_warehouse_id: toWh,
          sku_code: skuCode,
          quantity: qty,
          reason
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'Transfer failed'));
        return;
      }
      const data = await res.json();
      this.showToast(`Transfer #${data.transfer_number} executed for ${qty}x ${skuCode}`);
      this.closeModal();
      await this.reloadEntity('warehouseStocks');
      await this.reloadEntity('stockTransfers');
      await this.reloadEntity('skus');
      await this.reloadEntity('auditLogs');
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  openReceiveGRNModal(poId = '') {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '640px';

    const poList = (this.data.purchaseOrders || []).filter(p => p.status === 'ordered' || p.status === 'partially_received' || p.status === 'approved' || p.id === poId);
    const poOptions = poList.map(p => `<option value="${p.id}" ${p.id === poId ? 'selected' : ''}>${p.id} — Total: $${Number(p.total ?? p.total_amount ?? 0).toFixed(2)} (${p.status})</option>`).join('') || '<option value="po_01">po_01 — $2,250.00 (ordered)</option>';
    const whOptions = (this.data.warehouses || []).map(w => `<option value="${w.id}">${w.name} (${w.code})</option>`).join('');

    // Pre-populate items from selected PO
    const targetPO = poList.find(p => p.id === poId) || poList[0];
    let items = [];
    if (targetPO && targetPO.items_json) {
      try { items = JSON.parse(targetPO.items_json); } catch(e) {}
    }
    if (items.length === 0) {
      items = [{ sku_code: 'SKU-CORE-100', quantity: 20 }];
    }

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:10px; color:var(--text-main);">Receive Goods (GRN Generator)</h3>
      <p style="font-size:13px; color:var(--text-muted); margin-bottom:16px;">
        Issue a physical Goods Receipt Note. Supports partial receipts, auto-allocates to warehouse stock, and updates General Ledger.
      </p>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="grn-po" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Purchase Order</label>
            <select id="grn-po" class="form-input">
              ${poOptions}
            </select>
          </div>
          <div>
            <label for="grn-warehouse" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Destination Warehouse</label>
            <select id="grn-warehouse" class="form-input">
              ${whOptions}
            </select>
          </div>
        </div>

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="grn-carrier" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Carrier / Tracking #</label>
            <input type="text" id="grn-carrier" class="form-input" value="FedEx Express #8829-1002">
          </div>
          <div>
            <label for="grn-inspector" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Receiving Inspector / Agent</label>
            <input type="text" id="grn-inspector" class="form-input" value="Autonomous Receiving Agent">
          </div>
        </div>

        <div>
          <label style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase; margin-bottom:6px; display:block;">Physical Received Items</label>
          <table class="data-table" style="font-size:13px;" id="grn-items-table">
            <thead>
              <tr>
                <th>SKU Code</th>
                <th style="width:130px;">Received Qty</th>
                <th style="width:140px;">Storage Bin</th>
              </tr>
            </thead>
            <tbody id="grn-items-body">
              ${items.map((it, idx) => `
                <tr class="grn-item-row">
                  <td><input type="text" class="form-input grn-item-sku" style="padding:6px 8px; font-size:12.5px;" value="${it.sku_code || 'SKU-CORE-100'}"></td>
                  <td><input type="number" class="form-input grn-item-qty" style="padding:6px 8px; font-size:12.5px;" value="${it.quantity || 20}" min="1"></td>
                  <td><input type="text" class="form-input grn-item-bin" style="padding:6px 8px; font-size:12.5px;" value="A-01-0${idx + 1}"></td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>

        <label for="grn-notes" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Receiving Inspection Notes</label>
        <input type="text" id="grn-notes" class="form-input" value="Shipment verified in good physical condition; quality QA passed.">

        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitReceiveGRN()">
            <i data-lucide="package-check" class="w-4 h-4"></i> Issue GRN & Restock
          </button>
        </div>
      </div>
    `;
    modal.classList.add('active');
  },

  async submitReceiveGRN() {
    const poId = document.getElementById('grn-po')?.value;
    const whId = document.getElementById('grn-warehouse')?.value;
    const carrier = document.getElementById('grn-carrier')?.value.trim();
    const inspector = document.getElementById('grn-inspector')?.value.trim();
    const notes = document.getElementById('grn-notes')?.value.trim();

    const rows = document.querySelectorAll('.grn-item-row');
    const items = [];
    rows.forEach(r => {
      const sku = r.querySelector('.grn-item-sku')?.value.trim();
      const qty = parseInt(r.querySelector('.grn-item-qty')?.value || 0, 10);
      const bin = r.querySelector('.grn-item-bin')?.value.trim() || 'A-01-01';
      if (sku && qty > 0) {
        items.push({ sku_code: sku, quantity: qty, bin_location: bin });
      }
    });

    if (items.length === 0) {
      this.showToast('Please specify at least one item to receive');
      return;
    }

    try {
      const res = await fetch('/api/grns', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          po_id: poId,
          warehouse_id: whId,
          carrier_tracking: carrier,
          inspector_name: inspector,
          notes,
          items_json: JSON.stringify(items)
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'GRN failed'));
        return;
      }
      const data = await res.json();
      this.showToast(`GRN #${data.grn_number} created and inventory updated`);
      this.closeModal();
      await this.reloadEntity('grns');
      await this.reloadEntity('purchaseOrders');
      await this.reloadEntity('warehouseStocks');
      await this.reloadEntity('skus');
      await this.reloadEntity('summary');
      await this.reloadEntity('auditLogs');
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  openNewSupplierBillModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '540px';

    const supOptions = (this.data.suppliers || []).map(s => `<option value="${s.id}">${s.name}</option>`).join('');
    const poOptions = (this.data.purchaseOrders || []).map(p => `<option value="${p.id}">${p.id} ($${Number(p.total ?? p.total_amount ?? 0).toFixed(2)})</option>`).join('') || '<option value="po_01">po_01</option>';
    const grnOptions = (this.data.grns || []).map(g => `<option value="${g.id}">${g.grn_number} (${g.po_id})</option>`).join('') || '<option value="">No GRN recorded yet</option>';

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:10px; color:var(--text-main);">Add Supplier Bill</h3>
      <p style="font-size:13px; color:var(--text-muted); margin-bottom:16px;">
        Register a vendor invoice for Accounts Payable and 3-Way Matching reconciliation.
      </p>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="bill-number" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Bill # / Invoice Ref</label>
            <input type="text" id="bill-number" class="form-input" value="BILL-2026-${Math.floor(1000 + Math.random() * 9000)}">
          </div>
          <div>
            <label for="bill-supplier" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Vendor / Supplier</label>
            <select id="bill-supplier" class="form-input">
              ${supOptions}
            </select>
          </div>
        </div>

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="bill-po" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Target Purchase Order</label>
            <select id="bill-po" class="form-input">
              ${poOptions}
            </select>
          </div>
          <div>
            <label for="bill-grn" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Related GRN</label>
            <select id="bill-grn" class="form-input">
              ${grnOptions}
            </select>
          </div>
        </div>

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="bill-amount" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Billed Amount ($)</label>
            <input type="number" id="bill-amount" class="form-input" value="2250.00" step="0.5">
          </div>
          <div>
            <label for="bill-due" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Due Date</label>
            <input type="date" id="bill-due" class="form-input" value="2026-11-25">
          </div>
        </div>

        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitNewSupplierBill()">Register Supplier Bill</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
  },

  async submitNewSupplierBill() {
    const billNum = document.getElementById('bill-number')?.value.trim();
    const supId = document.getElementById('bill-supplier')?.value;
    const poId = document.getElementById('bill-po')?.value;
    const grnId = document.getElementById('bill-grn')?.value;
    const amount = parseFloat(document.getElementById('bill-amount')?.value || 0);
    const dueDate = document.getElementById('bill-due')?.value;

    if (!billNum) {
      this.showToast('Please enter a bill number');
      return;
    }

    try {
      const res = await fetch('/api/supplier-bills', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          bill_number: billNum,
          supplier_id: supId,
          po_id: poId,
          grn_id: grnId,
          amount,
          due_date: dueDate
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'Failed to register bill'));
        return;
      }
      this.showToast(`Supplier Bill ${billNum} recorded`);
      this.closeModal();
      await this.reloadEntity('supplierBills');
      await this.reloadEntity('summary');
      await this.reloadEntity('auditLogs');
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  async runThreeWayMatch(billId) {
    try {
      this.showToast('Running 3-Way Matching Engine...');
      const res = await fetch('/api/supplier-bills/match', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ bill_id: billId })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Match error: ' + (err.message || 'Reconciliation failed'));
        return;
      }
      const matchResult = await res.json();
      
      const modal = document.getElementById('action-modal');
      const content = document.getElementById('modal-content');
      modal.querySelector('.modal-window').style.maxWidth = '560px';

      const isMatched = matchResult.match_status === 'matched';
      content.innerHTML = `
        <div style="display:flex; align-items:center; gap:12px; margin-bottom:12px;">
          <div class="w-10 h-10 rounded-xl ${isMatched ? 'bg-emerald-100 text-emerald-700' : 'bg-amber-100 text-amber-700'} flex items-center justify-center">
            <i data-lucide="${isMatched ? 'check-circle' : 'alert-triangle'}" class="w-5 h-5"></i>
          </div>
          <div>
            <h3 style="font-size:18px; font-weight:800; color:var(--text-main);">3-Way Match Inspection</h3>
            <span class="badge ${isMatched ? 'badge-matched' : 'badge-variance-price'}">${(matchResult.match_status || matchResult.status || 'MATCHED').toUpperCase()}</span>
          </div>
        </div>

        <div style="background:var(--bg-tertiary); border:1px solid var(--border-subtle); border-radius:14px; padding:16px; margin-bottom:16px;">
          <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px; font-size:13px;">
            <div>
              <span class="text-slate-400">Bill Number:</span>
              <div class="font-bold text-slate-800">${matchResult.bill_number}</div>
            </div>
            <div>
              <span class="text-slate-400">Billed Total:</span>
              <div class="font-bold text-slate-800">$${matchResult.bill_amount.toFixed(2)}</div>
            </div>
            <div>
              <span class="text-slate-400">PO Contract Expected:</span>
              <div class="font-bold text-slate-800">$${matchResult.po_total.toFixed(2)}</div>
            </div>
            <div>
              <span class="text-slate-400">Variance Amount:</span>
              <div class="font-bold ${Math.abs(matchResult.variance_amount) > 0 ? 'text-rose-600' : 'text-emerald-600'}">
                $${matchResult.variance_amount.toFixed(2)}
              </div>
            </div>
          </div>
        </div>

        <div style="font-size:13px; color:var(--text-muted); line-height:1.5; margin-bottom:20px;">
          ${matchResult.discrepancy_reason ? `<strong>Details:</strong> ${matchResult.discrepancy_reason}` : 'All line items, contract pricing, and physical receipts match perfectly with zero variance.'}
          ${!isMatched ? `<br><br><span style="color:#D97706; font-weight:600;">⚠️ Discrepancy detected: this bill has been placed on hold and sent to the Human Approval Inbox.</span>` : ''}
        </div>

        <div style="display:flex; justify-content:flex-end;">
          <button class="btn-press btn btn-primary" onclick="app.closeModal()">Acknowledge</button>
        </div>
      `;
      modal.classList.add('active');
      if (window.lucide) lucide.createIcons();

      await this.reloadEntity('supplierBills');
      await this.reloadEntity('approvals');
      await this.reloadEntity('auditLogs');
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  openNewCreditNoteModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '560px';

    const invoiceOptions = (this.data.invoices || []).map(inv => `
      <option value="${inv.id}" data-customer="${inv.customer_id}" data-total="${inv.total_amount}" data-subtotal="${inv.subtotal}" data-tax="${inv.tax_amount}" data-currency="${inv.currency || 'USD'}">
        ${inv.invoice_number} — ${inv.customer_name || 'Client'} (${this.formatMoney(inv.total_amount, inv.currency || this.activeCurrency)})
      </option>
    `).join('') || '<option value="">No invoices found</option>';

    const custOptions = (this.data.customers || []).map(c => `<option value="${c.id}">${c.name}</option>`).join('') ||
      (this.data.invoices || []).map(inv => `<option value="${inv.customer_id}">${inv.customer_name || inv.customer_id}</option>`).join('') ||
      '<option value="cust_01">Primary Enterprise Client</option>';

    content.innerHTML = `
      <div style="display:flex; align-items:center; gap:12px; margin-bottom:12px;">
        <div class="w-10 h-10 rounded-xl bg-rose-100 text-rose-700 flex items-center justify-center shrink-0">
          <i data-lucide="file-minus" class="w-5 h-5"></i>
        </div>
        <div>
          <h3 style="font-size:18px; font-weight:800; color:var(--text-main);">Issue Credit Note</h3>
          <p style="font-size:13px; color:var(--text-muted);">Double-entry billing adjustment and sales reversal document.</p>
        </div>
      </div>

      <div style="display:flex; flex-direction:column; gap:12px;">
        <div>
          <label for="cn-invoice" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Select Related Invoice</label>
          <select id="cn-invoice" class="form-input" onchange="app.onCreditNoteInvoiceChange(this)">
            ${invoiceOptions}
          </select>
        </div>

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="cn-customer" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Customer</label>
            <select id="cn-customer" class="form-input">
              ${custOptions}
            </select>
          </div>
          <div>
            <label for="cn-reason" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Reason Code</label>
            <select id="cn-reason" class="form-input">
              <option value="billing_error">Billing / Pricing Error</option>
              <option value="goods_returned">Goods Damaged / Returned</option>
              <option value="discount_adjustment">Post-Sale Discount / Rebate</option>
              <option value="cancellation">Order Cancellation</option>
              <option value="goodwill">Goodwill Concession</option>
            </select>
          </div>
        </div>

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="cn-amount" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Subtotal Reversal Amount ($)</label>
            <input type="number" id="cn-amount" class="form-input" value="100.00" step="0.5" oninput="app.recalcCreditNoteTotal()">
          </div>
          <div>
            <label for="cn-tax" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Tax Reversal ($)</label>
            <input type="number" id="cn-tax" class="form-input" value="8.00" step="0.1" oninput="app.recalcCreditNoteTotal()">
          </div>
        </div>

        <div style="padding:10px 14px; background:var(--bg-tertiary); border:1px solid var(--border-subtle); border-radius:10px; display:flex; justify-content:space-between; align-items:center;">
          <span style="font-size:13px; font-weight:600; color:var(--text-muted);">Total Reversal Credited:</span>
          <span id="cn-total-display" style="font-size:16px; font-weight:800; color:#DC2626;">$108.00</span>
        </div>

        <label for="cn-notes" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Memo / Audit Reason</label>
        <input type="text" id="cn-notes" class="form-input" value="Credit adjustment per mutual agreement with client">

        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitCreditNote()">
            <i data-lucide="check" class="w-4 h-4"></i> Issue & Post to Ledger
          </button>
        </div>
      </div>
    `;

    modal.classList.add('active');
    if (window.lucide) lucide.createIcons();
    this.onCreditNoteInvoiceChange(document.getElementById('cn-invoice'));
  },

  onCreditNoteInvoiceChange(selectEl) {
    if (!selectEl || !selectEl.options || selectEl.selectedIndex < 0) return;
    const opt = selectEl.options[selectEl.selectedIndex];
    if (!opt) return;
    const custId = opt.getAttribute('data-customer');
    const subtotal = parseFloat(opt.getAttribute('data-subtotal') || 0);
    const tax = parseFloat(opt.getAttribute('data-tax') || 0);

    const custEl = document.getElementById('cn-customer');
    if (custEl && custId) custEl.value = custId;

    const amtEl = document.getElementById('cn-amount');
    const taxEl = document.getElementById('cn-tax');
    if (amtEl && subtotal > 0) amtEl.value = (subtotal * 0.5).toFixed(2);
    if (taxEl && tax > 0) taxEl.value = (tax * 0.5).toFixed(2);
    this.recalcCreditNoteTotal();
  },

  recalcCreditNoteTotal() {
    const amt = parseFloat(document.getElementById('cn-amount')?.value || 0);
    const tax = parseFloat(document.getElementById('cn-tax')?.value || 0);
    const total = amt + tax;
    const display = document.getElementById('cn-total-display');
    if (display) display.textContent = `$${total.toFixed(2)}`;
  },

  async submitCreditNote() {
    const invId = document.getElementById('cn-invoice')?.value;
    const custId = document.getElementById('cn-customer')?.value;
    const reason = document.getElementById('cn-reason')?.value || 'billing_error';
    const subtotal = parseFloat(document.getElementById('cn-amount')?.value || 0);
    const tax = parseFloat(document.getElementById('cn-tax')?.value || 0);
    const total = subtotal + tax;
    const memo = document.getElementById('cn-notes')?.value || '';

    if (total <= 0) {
      this.showToast('Credit note amount must be greater than zero');
      return;
    }

    try {
      this.showToast('Posting Credit Note & General Ledger Reversal...');
      const res = await fetch('/api/credit-notes', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          invoice_id: invId,
          customer_id: custId,
          reason,
          subtotal,
          tax_amount: tax,
          total_amount: total,
          currency: this.activeCurrency,
          items_json: JSON.stringify([{ description: memo || reason, amount: subtotal, tax }])
        })
      });

      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'Failed to issue credit note'));
        return;
      }

      const data = await res.json();
      this.showToast(`Credit Note #${data.credit_note_number} successfully posted to Ledger`);
      this.closeModal();

      await this.reloadEntity('creditNotes');
      await this.reloadEntity('accounts');
      await this.reloadEntity('journalEntries');
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
      { title: 'Add New Sales Deal', tag: 'Action', action: () => { app.closeCommandPalette(); app.openNewDealModal(); } },
      { title: 'Add Team Member (Employee)', tag: 'Action', action: () => { app.closeCommandPalette(); app.openNewEmployeeModal(); } },
      { title: 'Schedule Employee Shift', tag: 'Action', action: () => { app.closeCommandPalette(); app.openNewShiftModal(); } },
      { title: 'Create Project Task', tag: 'Action', action: () => { app.closeCommandPalette(); app.openNewTaskModal(); } },
      { title: 'Add New Warehouse', tag: 'Action', action: () => { app.closeCommandPalette(); app.openNewWarehouseModal(); } },
      { title: 'Transfer Warehouse Stock', tag: 'Action', action: () => { app.closeCommandPalette(); app.openStockTransferModal(); } },
      { title: 'Receive Goods (GRN)', tag: 'Action', action: () => { app.closeCommandPalette(); app.openReceiveGRNModal(); } },
      { title: 'Add Supplier Bill & 3-Way Match', tag: 'Action', action: () => { app.closeCommandPalette(); app.openNewSupplierBillModal(); } }
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

window.app = app;

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
  app.init();
});
