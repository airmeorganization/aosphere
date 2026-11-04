// ==========================================================================
// AO Sphere — Unified Operating System for Modern Business
// Client Application Engine (Minimal & Modern Theme)
// ==========================================================================

const app = {
  activeTab: 'dashboard',
  activeERPSubTab: 'skus',
  activeFinanceSubTab: 'invoices',
  activeCurrency: localStorage.getItem('aosphere_currency') || 'INR',
  activeCRMSubTab: 'kanban',
  activePipelineId: 'pipe_std_org_default',
  activeHRSubTab: 'roster',
  selectedTimeclockEmpId: '',
  activePayrollPeriod: new Date().toISOString().slice(0, 7),
  activeReportsSubTab: 'kpis',
  activeComplianceSubTab: 'tenancy',
  reportsFilter: {
    pnlStart: new Date(new Date().getFullYear(), 0, 1).toISOString().slice(0, 10),
    pnlEnd: new Date().toISOString().slice(0, 10),
    balanceSheetAsOf: new Date().toISOString().slice(0, 10),
    cashFlowStart: new Date(new Date().getFullYear(), 0, 1).toISOString().slice(0, 10),
    cashFlowEnd: new Date().toISOString().slice(0, 10),
    agingAsOf: new Date().toISOString().slice(0, 10),
    agingType: 'ar',
    inventoryMode: 'sku',
    exportEntity: 'pnl',
    exportFormat: 'pdf',
    exportStart: new Date(new Date().getFullYear(), 0, 1).toISOString().slice(0, 10),
    exportEnd: new Date().toISOString().slice(0, 10)
  },
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
    customers: [],
    deals: [],
    pipelines: [],
    decayAlerts: [],
    inboundLeads: [],
    emailOutbox: [],
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
    shiftConflicts: [],
    timeclockStatus: null,
    payrollPreview: null,
    payrollRuns: [],
    payslips: [],
    leavePolicies: [],
    leaveRequests: [],
    hrDocuments: [],
    tasks: [],
    listings: [],
    reviews: [],
    approvals: [],
    auditLogs: [],
    tools: [],
    reportsKPIs: null,
    reportsPnL: null,
    reportsBalanceSheet: null,
    reportsCashFlow: null,
    reportsAgingAR: null,
    reportsAgingAP: null,
    reportsInventoryValuation: null,
    reportsInventoryTurnover: null,
    reportsSalesVelocity: null,
    organizations: [],
    userInvitations: [],
    ssoConfigs: [],
    customRoles: [],
    auditVaultBlocks: [],
    auditVaultIntegrity: null,
    privacyRequests: [],
    soc2Reviews: []
  },
  sse: null,

  init() {
    this.setupTheme();
    this.setupCurrency();
    this.setupShortcuts();
    this.setupSSE();
    this.loadOrganizations();
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

  setCRMSubTab(subTab) {
    this.activeCRMSubTab = subTab;
    this.render();
    if (window.lucide) lucide.createIcons();
  },

  setCRMPipeline(pipeId) {
    this.activePipelineId = pipeId;
    this.render();
    if (window.lucide) lucide.createIcons();
  },

  setHRSubTab(subTab) {
    this.activeHRSubTab = subTab;
    this.render();
    if (window.lucide) lucide.createIcons();
  },

  async setReportsSubTab(subTab) {
    this.activeReportsSubTab = subTab;
    await this.ensureReportData(subTab);
    this.render();
    if (window.lucide) lucide.createIcons();
  },

  setReportsFilter(key, val) {
    this.reportsFilter[key] = val;
  },

  async setSelectedTimeclockEmployee(empId) {
    this.selectedTimeclockEmpId = empId;
    await this.reloadTimeclockStatus();
    this.render();
    if (window.lucide) lucide.createIcons();
  },

  async reloadTimeclockStatus() {
    if (!this.selectedTimeclockEmpId && this.data.employees && this.data.employees.length > 0) {
      this.selectedTimeclockEmpId = this.data.employees[0].id;
    }
    if (!this.selectedTimeclockEmpId) return;
    try {
      const res = await fetch('/api/timeclock/status?employee_id=' + encodeURIComponent(this.selectedTimeclockEmpId));
      if (res.ok) {
        this.data.timeclockStatus = await res.json();
      }
    } catch (err) {
      console.warn('Failed to reload timeclock status:', err);
    }
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
        shift_updated: ['shifts', 'shiftConflicts'],
        shift_created: ['shifts', 'shiftConflicts'],
        shift_conflict: ['shiftConflicts'],
        task_updated: ['tasks'],
        task_created: ['tasks'],
        employee_created: ['employees'],
        employee_updated: ['employees'],
        timeclock_event: ['timeclockStatus', 'shifts', 'shiftConflicts'],
        payroll_run: ['payrollRuns', 'payslips', 'accounts', 'journalEntries', 'summary'],
        leave_requested: ['leaveRequests'],
        leave_reviewed: ['leaveRequests'],
        hr_document_uploaded: ['hrDocuments'],
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
      customers: '/api/customers',
      deals: '/api/deals',
      pipelines: '/api/pipelines',
      decayAlerts: '/api/deals/decay-alerts',
      inboundLeads: '/api/leads',
      emailOutbox: '/api/emails/outbox',
      invoices: '/api/invoices',
      creditNotes: '/api/credit-notes',
      accounts: '/api/ledger/accounts',
      journalEntries: '/api/ledger/entries',
      currencies: '/api/currencies',
      summary: '/api/financial-summary',
      employees: '/api/employees',
      shifts: '/api/shifts',
      shiftConflicts: '/api/shifts/conflicts',
      payrollRuns: '/api/payroll/history',
      payslips: '/api/payroll/payslips',
      leavePolicies: '/api/leaves/policies',
      leaveRequests: '/api/leaves/requests',
      hrDocuments: '/api/hr/documents',
      tasks: '/api/tasks',
      listings: '/api/listings',
      reviews: '/api/reviews',
      approvals: '/api/approvals',
      auditLogs: '/api/audit-logs'
    };
    if (name === 'timeclockStatus') {
      await this.reloadTimeclockStatus();
      this.render();
      return;
    }
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
        customers,
        deals,
        pipelines,
        decayAlerts,
        inboundLeads,
        emailOutbox,
        invoices,
        creditNotes,
        accounts,
        journalEntries,
        currenciesResp,
        summary,
        employees,
        shifts,
        shiftConflicts,
        payrollRuns,
        payslips,
        leavePolicies,
        leaveRequests,
        hrDocuments,
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
        fetch('/api/customers').then(r => r.json()).catch(() => []),
        fetch('/api/deals').then(r => r.json()).catch(() => []),
        fetch('/api/pipelines').then(r => r.json()).catch(() => []),
        fetch('/api/deals/decay-alerts').then(r => r.json()).catch(() => []),
        fetch('/api/leads').then(r => r.json()).catch(() => []),
        fetch('/api/emails/outbox').then(r => r.json()).catch(() => []),
        fetch('/api/invoices').then(r => r.json()).catch(() => []),
        fetch('/api/credit-notes').then(r => r.json()).catch(() => []),
        fetch('/api/ledger/accounts').then(r => r.json()).catch(() => []),
        fetch('/api/ledger/entries').then(r => r.json()).catch(() => []),
        fetch('/api/currencies').then(r => r.json()).catch(() => null),
        fetch('/api/financial-summary').then(r => r.json()).catch(() => null),
        fetch('/api/employees').then(r => r.json()).catch(() => []),
        fetch('/api/shifts').then(r => r.json()).catch(() => []),
        fetch('/api/shifts/conflicts').then(r => r.json()).catch(() => []),
        fetch('/api/payroll/history').then(r => r.json()).catch(() => []),
        fetch('/api/payroll/payslips').then(r => r.json()).catch(() => []),
        fetch('/api/leaves/policies').then(r => r.json()).catch(() => []),
        fetch('/api/leaves/requests').then(r => r.json()).catch(() => []),
        fetch('/api/hr/documents').then(r => r.json()).catch(() => []),
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
      this.data.customers = customers;
      this.data.deals = deals;
      this.data.pipelines = pipelines;
      this.data.decayAlerts = decayAlerts;
      this.data.inboundLeads = inboundLeads;
      this.data.emailOutbox = emailOutbox;
      if (pipelines && pipelines.length > 0 && (!this.activePipelineId || !pipelines.find(p => p.id === this.activePipelineId))) {
        this.activePipelineId = pipelines[0].id;
      }
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
      this.data.shiftConflicts = shiftConflicts || [];
      this.data.payrollRuns = payrollRuns || [];
      this.data.payslips = payslips || [];
      this.data.leavePolicies = leavePolicies || [];
      this.data.leaveRequests = leaveRequests || [];
      this.data.hrDocuments = hrDocuments || [];
      this.data.tasks = tasks;
      this.data.listings = listings;
      this.data.reviews = reviews;
      this.data.approvals = approvals;
      this.data.auditLogs = auditLogs;

      if (employees && employees.length > 0 && !this.selectedTimeclockEmpId) {
        this.selectedTimeclockEmpId = employees[0].id;
      }
      if (this.selectedTimeclockEmpId) {
        await this.reloadTimeclockStatus();
      }

      const picker = document.getElementById('global-currency-picker');
      if (picker) picker.value = this.activeCurrency;

      try {
        const kpiRes = await fetch('/api/reports/executive-kpis');
        if (kpiRes.ok) this.data.reportsKPIs = await kpiRes.json();
      } catch (e) {}

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
    if (tab === 'reports') {
      this.ensureReportData(this.activeReportsSubTab);
    }
    if (tab === 'agents') {
      this.ensureAgentPlatformData(this.activeAgentSubTab);
    }
    if (tab === 'compliance') {
      this.ensureComplianceData(this.activeComplianceSubTab);
    }
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
      case 'reports':
        main.innerHTML = this.renderReports();
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
      case 'agents':
        main.innerHTML = this.renderAgentPlatform();
        break;
      case 'compliance':
        main.innerHTML = this.renderCompliance();
        break;
      case 'backups':
        main.innerHTML = this.renderBackups();
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
  // ---------------- Module 3: CRM & Multi-Pipeline Visual Operations ----------------
  renderCRM() {
    const subTab = this.activeCRMSubTab || 'kanban';

    let actionButton = '';
    if (subTab === 'kanban') {
      const pipes = this.data.pipelines || [];
      const pipeOptions = pipes.map(p => `<option value="${p.id}" ${p.id === this.activePipelineId ? 'selected' : ''}>${p.name}</option>`).join('');
      actionButton = `
        <div style="display:flex; align-items:center; gap:10px;">
          <select class="form-input" style="width:230px; font-weight:700; height:38px;" onchange="app.setCRMPipeline(this.value)">
            ${pipeOptions}
          </select>
          <button class="btn-press btn btn-primary" onclick="app.openNewDealModal()">
            <i data-lucide="plus" class="w-4 h-4"></i> Add Deal
          </button>
        </div>
      `;
    } else if (subTab === 'customers') {
      actionButton = `
        <button class="btn-press btn btn-primary" onclick="app.openNewCustomerModal()">
          <i data-lucide="user-plus" class="w-4 h-4"></i> Add Customer
        </button>
      `;
    } else if (subTab === 'leads') {
      actionButton = `
        <button class="btn-press btn btn-primary" onclick="app.simulateInboundLeadModal()">
          <i data-lucide="inbox" class="w-4 h-4"></i> Ingest Inbound Lead
        </button>
      `;
    } else if (subTab === 'outbox') {
      actionButton = `
        <button class="btn-press btn btn-secondary" onclick="app.reloadEntity('emailOutbox'); app.showToast('Outbox refreshed');">
          <i data-lucide="refresh-cw" class="w-4 h-4"></i> Refresh Outbox
        </button>
      `;
    }

    let subTabContent = '';
    if (subTab === 'kanban') {
      subTabContent = this.renderCRMKanban();
    } else if (subTab === 'customers') {
      subTabContent = this.renderCustomer360List();
    } else if (subTab === 'leads') {
      subTabContent = this.renderInboundLeads();
    } else if (subTab === 'outbox') {
      subTabContent = this.renderEmailOutbox();
    }

    return `
      <div class="page-header">
        <div>
          <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-100 text-emerald-700 text-[11px] font-bold uppercase tracking-widest mb-3">
            <i data-lucide="users" class="w-3.5 h-3.5"></i> CRM & Omnichannel Operations
          </div>
          <h1 class="page-title">Customer 360 & Sales Pipelines</h1>
          <p class="page-sub">Omnichannel customer hub, multi-pipeline deal boards, decay alerts, and lead capture.</p>
        </div>
        <div class="page-actions">
          ${actionButton}
        </div>
      </div>

      <div class="subnav-bar">
        <button class="subnav-pill ${subTab === 'kanban' ? 'active' : ''}" onclick="app.setCRMSubTab('kanban')">
          <i data-lucide="kanban" class="w-4 h-4"></i> Sales Pipelines
        </button>
        <button class="subnav-pill ${subTab === 'customers' ? 'active' : ''}" onclick="app.setCRMSubTab('customers')">
          <i data-lucide="user-check" class="w-4 h-4"></i> Customer 360 Hub
        </button>
        <button class="subnav-pill ${subTab === 'leads' ? 'active' : ''}" onclick="app.setCRMSubTab('leads')">
          <i data-lucide="arrow-down-left" class="w-4 h-4"></i> Inbound Leads (${(this.data.inboundLeads || []).filter(l => l.status === 'new').length} New)
        </button>
        <button class="subnav-pill ${subTab === 'outbox' ? 'active' : ''}" onclick="app.setCRMSubTab('outbox')">
          <i data-lucide="send" class="w-4 h-4"></i> Email Outbox
        </button>
      </div>

      ${subTabContent}
    `;
  },

  renderCRMKanban() {
    const pipes = this.data.pipelines || [];
    let activePipe = pipes.find(p => p.id === this.activePipelineId);
    if (!activePipe && pipes.length > 0) {
      activePipe = pipes[0];
      this.activePipelineId = activePipe.id;
    }

    const stages = (activePipe && activePipe.stages && activePipe.stages.length > 0)
      ? activePipe.stages
      : [
          { id: 'lead', name: 'New Leads', probability: 20, decay_threshold_days: 7 },
          { id: 'qualified', name: 'Qualified', probability: 40, decay_threshold_days: 10 },
          { id: 'proposal', name: 'Proposal', probability: 60, decay_threshold_days: 14 },
          { id: 'negotiation', name: 'Negotiation', probability: 80, decay_threshold_days: 10 },
          { id: 'won', name: 'Won', probability: 100, decay_threshold_days: 30 }
        ];

    const alerts = this.data.decayAlerts || [];
    const pipeAlerts = alerts.filter(a => !activePipe || a.pipeline_id === activePipe.id || !a.pipeline_id);

    let decayBanner = '';
    if (pipeAlerts.length > 0) {
      decayBanner = `
        <div class="deal-decay-banner">
          <div>
            <div style="font-size:13.5px; font-weight:800; color:#B45309; display:flex; align-items:center; gap:8px;">
              <i data-lucide="alert-triangle" class="w-4 h-4 text-amber-600"></i>
              Automated Deal Decay Warning: ${pipeAlerts.length} Stagnant Deal${pipeAlerts.length > 1 ? 's' : ''} in ${activePipe ? activePipe.name : 'Pipeline'}
            </div>
            <div style="font-size:12px; color:#92400E; margin-top:3px;">
              Deals that have stayed in a stage beyond configured policy decay thresholds:
            </div>
          </div>
          <div style="display:flex; gap:8px; flex-wrap:wrap;">
            ${pipeAlerts.map(a => `
              <span class="decay-badge" style="cursor:pointer;" onclick="app.openCustomer360Modal('${a.customer_id}')">
                ⚠️ <strong>${a.deal_title}</strong>: ${a.days_in_stage}d in ${a.stage_name} (Max ${a.decay_threshold_days}d)
              </span>
            `).join('')}
          </div>
        </div>
      `;
    }

    return `
      ${decayBanner}
      <div class="kanban-board">
        ${stages.map(stage => {
          const deals = (this.data.deals || []).filter(d => {
            const matchesPipe = !d.pipeline_id || !activePipe || d.pipeline_id === activePipe.id;
            return matchesPipe && d.stage === stage.id;
          });
          const colTotal = deals.reduce((sum, d) => sum + (parseFloat(d.value) || 0), 0);
          return `
            <div class="kanban-column" data-stage="${stage.id}">
              <div class="kanban-col-header">
                <div>
                  <span class="col-title">${stage.name}</span>
                  <div style="font-size:11.5px; color:var(--text-muted); font-weight:700; margin-top:2px;">
                    ${this.formatMoney(colTotal)} • ${stage.probability}% prob
                  </div>
                  <div style="font-size:10px; color:#94A3B8; font-weight:600;">Decay: ${stage.decay_threshold_days}d limit</div>
                </div>
                <span class="col-badge">${deals.length}</span>
              </div>
              <div class="kanban-cards-area" id="col-${stage.id}">
                ${deals.map(deal => {
                  const alert = alerts.find(a => a.deal_id === deal.id);
                  return `
                    <div class="deal-card" draggable="true" data-deal-id="${deal.id}">
                      <div class="deal-title">${deal.title}</div>
                      <div class="deal-customer" style="cursor:pointer; color:var(--brand-blue);" onclick="event.stopPropagation(); app.openCustomer360Modal('${deal.customer_id}')">
                        <i data-lucide="building" class="w-3 h-3 inline"></i> ${deal.customer_name || 'Client'} • ${deal.customer_company || ''}
                      </div>
                      ${alert ? `
                        <div class="decay-badge">
                          <i data-lucide="clock" class="w-3 h-3"></i> Stagnant (${alert.days_in_stage}d > ${alert.decay_threshold_days}d limit)
                        </div>
                      ` : ''}
                      <div class="deal-footer" style="margin-top:8px;">
                        <span class="deal-value">${this.formatMoney(deal.value)}</span>
                        <span style="font-size:11px; font-weight:700; color:#64748B; background:var(--bg-tertiary); padding:2px 6px; border-radius:4px;">${deal.probability || stage.probability}%</span>
                      </div>
                      <div style="display:flex; gap:6px; margin-top:8px;">
                        <button class="btn-press btn btn-secondary btn-sm" style="flex:1; font-size:11px; height:28px;" onclick="event.stopPropagation(); app.openCustomer360Modal('${deal.customer_id}')">
                          <i data-lucide="user" class="w-3.5 h-3.5 text-blue-500"></i> 360 Hub
                        </button>
                        <button class="btn-press btn btn-secondary btn-sm" style="flex:1; font-size:11px; height:28px;" onclick="event.stopPropagation(); app.convertDealToInvoice('${deal.id}')">
                          <i data-lucide="receipt" class="w-3.5 h-3.5 text-emerald-500"></i> Invoice
                        </button>
                      </div>
                    </div>
                  `;
                }).join('')}
              </div>
            </div>
          `;
        }).join('')}
      </div>
    `;
  },

  renderCustomer360List() {
    const customers = this.data.customers || [];
    return `
      <div class="table-container mockup-item">
        <div class="table-filter-bar">
          <input type="text" class="table-filter-input" placeholder="Search customers by name, company, email..." oninput="app.filterTable('customers-table', this.value)">
          <span class="text-[12px] text-slate-400 font-medium">Active Accounts: <strong>${customers.length}</strong></span>
        </div>
        <table class="data-table" id="customers-table">
          <thead>
            <tr>
              <th>Customer Name</th>
              <th>Company</th>
              <th>Contact Info</th>
              <th>Status</th>
              <th>Lifetime Value (${this.activeCurrency})</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            ${customers.map(c => `
              <tr>
                <td><strong>${c.name}</strong></td>
                <td><span class="text-slate-600 font-medium">${c.company || '—'}</span></td>
                <td>
                  <div style="font-size:12px; color:var(--text-muted);">${c.email || '—'}</div>
                  <div style="font-size:11px; color:#94A3B8;">${c.phone || ''}</div>
                </td>
                <td>
                  <span class="badge" style="background:#ECFDF5; color:#047857; border:1px solid #A7F3D0;">
                    ${(c.status || 'ACTIVE').toUpperCase()}
                  </span>
                </td>
                <td><strong style="color:#059669; font-size:14px;">${this.formatMoney(c.lifetime_value || 0)}</strong></td>
                <td>
                  <button class="btn btn-secondary btn-sm" onclick="app.openCustomer360Modal('${c.id}')">
                    <i data-lucide="layout-dashboard" class="w-3.5 h-3.5 text-blue-500"></i> Open 360 View
                  </button>
                </td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    `;
  },

  renderInboundLeads() {
    const leads = this.data.inboundLeads || [];
    return `
      <div class="table-container mockup-item">
        <div class="table-filter-bar">
          <input type="text" class="table-filter-input" placeholder="Search inbound leads..." oninput="app.filterTable('leads-table', this.value)">
          <div style="display:flex; align-items:center; gap:12px;">
            <span class="text-[12px] text-slate-400 font-medium">Public Webhook: <code class="mono-code">/api/v1/leads/capture</code></span>
            <span class="text-[12px] text-slate-400 font-medium">Total Inbound: <strong>${leads.length}</strong></span>
          </div>
        </div>
        <table class="data-table" id="leads-table">
          <thead>
            <tr>
              <th>Lead Name</th>
              <th>Company</th>
              <th>Source</th>
              <th>Contact</th>
              <th>Est. Value</th>
              <th>Captured At</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            ${leads.map(lead => `
              <tr>
                <td><strong>${lead.name}</strong></td>
                <td><span class="text-slate-600 font-medium">${lead.company || '—'}</span></td>
                <td><span class="mono-code" style="font-size:11px;">${lead.source || 'website'}</span></td>
                <td>
                  <div style="font-size:12px; color:var(--text-muted);">${lead.email || '—'}</div>
                  <div style="font-size:11px; color:#94A3B8;">${lead.phone || ''}</div>
                </td>
                <td><strong>${this.formatMoney(lead.estimated_value || 0)}</strong></td>
                <td class="text-slate-500 text-[12px]">${(lead.created_at || '').slice(0, 19).replace('T', ' ')}</td>
                <td>
                  <span class="badge" style="background:${lead.status === 'converted' ? '#ECFDF5' : '#EFF6FF'}; color:${lead.status === 'converted' ? '#047857' : '#1D4ED8'}; border:1px solid ${lead.status === 'converted' ? '#A7F3D0' : '#BFDBFE'};">
                    ${(lead.status || 'NEW').toUpperCase()}
                  </span>
                </td>
                <td>
                  ${lead.status !== 'converted' ? `
                    <button class="btn btn-primary btn-sm" onclick="app.convertLeadAction('${lead.id}')">
                      <i data-lucide="user-check" class="w-3.5 h-3.5"></i> 1-Click Convert
                    </button>
                  ` : `
                    <span class="text-emerald-600 font-bold text-[12px]">
                      <i data-lucide="check" class="w-3.5 h-3.5 inline"></i> Converted
                    </span>
                  `}
                </td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    `;
  },

  renderEmailOutbox() {
    const emails = this.data.emailOutbox || [];
    return `
      <div class="table-container mockup-item">
        <div class="table-filter-bar">
          <input type="text" class="table-filter-input" placeholder="Search email logs..." oninput="app.filterTable('outbox-table', this.value)">
          <span class="text-[12px] text-slate-400 font-medium">Transactional Emails Dispatched: <strong>${emails.length}</strong></span>
        </div>
        <table class="data-table" id="outbox-table">
          <thead>
            <tr>
              <th>Dispatched At</th>
              <th>Recipient</th>
              <th>Subject</th>
              <th>Status</th>
              <th>Client Payment Portal Link</th>
            </tr>
          </thead>
          <tbody>
            ${emails.map(e => `
              <tr>
                <td class="text-slate-500 text-[12px]">${(e.sent_at || e.created_at || '').slice(0, 19).replace('T', ' ')}</td>
                <td>
                  <strong>${e.recipient_name || 'Client'}</strong>
                  <div style="font-size:11.5px; color:var(--text-muted);">${e.recipient_email}</div>
                </td>
                <td class="font-medium">${e.subject}</td>
                <td>
                  <span class="badge" style="background:#ECFDF5; color:#047857; border:1px solid #A7F3D0;">
                    ${(e.status || 'DELIVERED').toUpperCase()}
                  </span>
                </td>
                <td>
                  ${e.portal_url ? `
                    <a href="${e.portal_url}" target="_blank" class="btn btn-secondary btn-sm" style="display:inline-flex; align-items:center; gap:4px;">
                      <i data-lucide="external-link" class="w-3.5 h-3.5 text-blue-500"></i> Open Client Portal
                    </a>
                  ` : '—'}
                </td>
              </tr>
            `).join('')}
          </tbody>
        </table>
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
            body: JSON.stringify({ deal_id: dealId, stage: targetStage, pipeline_id: app.activePipelineId })
          });
          await app.reloadEntity('deals');
          await app.reloadEntity('decayAlerts');
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
                    <div style="display:flex; gap:6px; align-items:center;">
                      <a href="/api/invoices/pdf?id=${inv.id}&currency=${app.activeCurrency}" target="_blank" class="btn-pdf" title="Download Vector PDF Invoice">
                        <i data-lucide="file-down" class="w-3.5 h-3.5"></i> PDF
                      </a>
                      <button class="btn btn-secondary btn-sm" onclick="app.sendInvoiceEmailAction('${inv.id}')" title="Dispatch Client Self-Service Portal Email">
                        <i data-lucide="mail" class="w-3.5 h-3.5 text-blue-500"></i> Email Link
                      </button>
                    </div>
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

  // ---------------- Module 5: HRMS, Shifts & Automated Payroll ----------------
  renderHR() {
    const subTab = this.activeHRSubTab || 'roster';
    let subTabContent = '';

    if (subTab === 'roster') {
      subTabContent = this.renderHRRoster();
    } else if (subTab === 'timeclock') {
      subTabContent = this.renderHRTimeclock();
    } else if (subTab === 'employees') {
      subTabContent = this.renderHREmployees();
    } else if (subTab === 'payroll') {
      subTabContent = this.renderHRPayroll();
    } else if (subTab === 'leaves') {
      subTabContent = this.renderHRLeaves();
    } else if (subTab === 'documents') {
      subTabContent = this.renderHRDocuments();
    }

    return `
      <div class="page-header">
        <div>
          <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-100 text-emerald-700 text-[11px] font-bold uppercase tracking-widest mb-3">
            <i data-lucide="users" class="w-3.5 h-3.5"></i> Human Resources & Payroll
          </div>
          <h1 class="page-title">Workforce, Shift Scheduling & Payroll</h1>
          <p class="page-sub">Comprehensive workforce management, digital timeclock punches, automated double-entry payroll, and role-gated compliance vault.</p>
        </div>
        <div class="page-actions">
          <button class="btn-press btn btn-secondary" onclick="app.openNewEmployeeModal()">
            <i data-lucide="user-plus" class="w-4 h-4 text-brand-blue"></i> Onboard Employee
          </button>
          <button class="btn-press btn btn-primary" onclick="app.openNewShiftModal()">
            <i data-lucide="calendar-plus" class="w-4 h-4"></i> Schedule Shift
          </button>
        </div>
      </div>

      <!-- HR & Payroll Sub-Navigation Pills -->
      <div class="subnav-bar">
        <button class="subnav-pill ${subTab === 'roster' ? 'active' : ''}" onclick="app.setHRSubTab('roster')">
          <i data-lucide="calendar" class="w-4 h-4"></i> Shift Roster & Conflicts (${(this.data.shifts || []).length})
        </button>
        <button class="subnav-pill ${subTab === 'timeclock' ? 'active' : ''}" onclick="app.setHRSubTab('timeclock')">
          <i data-lucide="clock" class="w-4 h-4"></i> Digital Timeclock
        </button>
        <button class="subnav-pill ${subTab === 'employees' ? 'active' : ''}" onclick="app.setHRSubTab('employees')">
          <i data-lucide="user-check" class="w-4 h-4"></i> Employee Directory (${(this.data.employees || []).length})
        </button>
        <button class="subnav-pill ${subTab === 'payroll' ? 'active' : ''}" onclick="app.setHRSubTab('payroll')">
          <i data-lucide="banknote" class="w-4 h-4"></i> Automated Payroll (${(this.data.payrollRuns || []).length})
        </button>
        <button class="subnav-pill ${subTab === 'leaves' ? 'active' : ''}" onclick="app.setHRSubTab('leaves')">
          <i data-lucide="plane" class="w-4 h-4"></i> Leave & PTO (${(this.data.leaveRequests || []).length})
        </button>
        <button class="subnav-pill ${subTab === 'documents' ? 'active' : ''}" onclick="app.setHRSubTab('documents')">
          <i data-lucide="shield-check" class="w-4 h-4"></i> HR Compliance Vault (${(this.data.hrDocuments || []).length})
        </button>
      </div>

      ${subTabContent}
    `;
  },

  renderHRRoster() {
    const conflicts = this.data.shiftConflicts || [];
    const conflictHTML = conflicts.length > 0 ? `
      <div class="conflict-alert-box">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2 font-bold text-red-800 text-[14px]">
            <i data-lucide="alert-triangle" class="w-4 h-4 text-red-600"></i>
            Scheduling Conflicts Detected (${conflicts.length})
          </div>
          <span class="text-xs text-red-700 font-medium">Automatic Labor Policy Enforcement</span>
        </div>
        ${conflicts.map(c => `
          <div class="conflict-item">
            <div class="flex items-center gap-2">
              <span class="${c.conflict_type === 'overtime_threshold' ? 'conflict-badge-warn' : 'conflict-badge-error'}">
                ${c.conflict_type.replace(/_/g, ' ')}
              </span>
              <strong class="text-slate-900">${c.employee_name || 'Employee'}:</strong>
              <span class="text-slate-700">${c.description}</span>
            </div>
            <span class="text-xs text-slate-500 font-mono">${c.shift_id_1} ${c.shift_id_2 ? '↔ ' + c.shift_id_2 : ''}</span>
          </div>
        `).join('')}
      </div>
    ` : `
      <div style="background:#ECFDF5; border:1px solid #A7F3D0; border-radius:12px; padding:12px 18px; margin-bottom:20px; display:flex; align-items:center; gap:10px; color:#065F46; font-size:13px; font-weight:600;">
        <i data-lucide="check-circle-2" class="w-4 h-4 text-emerald-600"></i>
        <span>Workforce Shift Compliance: Zero double-booking, rest period (&lt; 8h), or overtime (&gt; 40h) violations detected.</span>
      </div>
    `;

    return `
      ${conflictHTML}
      <div class="table-container mockup-item">
        <div class="table-filter-bar">
          <span class="font-bold text-[15px] text-slate-800">Master Shift Roster</span>
          <input type="text" class="table-filter-input" placeholder="Search shifts by employee, station..." oninput="app.filterTable('shifts-table', this.value)">
        </div>
        <table class="data-table" id="shifts-table">
          <thead>
            <tr>
              <th>Employee</th>
              <th>Shift Date</th>
              <th>Time Window</th>
              <th>Station</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            ${(this.data.shifts || []).length === 0 ? `<tr><td colspan="6" class="text-center py-6 text-slate-400">No shifts scheduled. Click "+ Schedule Shift" above to assign working hours.</td></tr>` :
              this.data.shifts.map(sh => `
                <tr>
                  <td><strong>${sh.employee_name}</strong></td>
                  <td class="font-mono text-slate-600 text-xs">${sh.shift_date || 'Today'}</td>
                  <td class="font-medium text-slate-500">${sh.start_time} — ${sh.end_time}</td>
                  <td><span class="badge" style="background:#F1F5F9; color:#475569;">${sh.station}</span></td>
                  <td>
                    <span class="badge" style="background:${sh.status === 'clocked_in' ? '#ECFDF5' : '#EFF6FF'}; color:${sh.status === 'clocked_in' ? '#047857' : '#1D4ED8'}; border:1px solid ${sh.status === 'clocked_in' ? '#A7F3D0' : '#BFDBFE'};">
                      ${sh.status.replace(/_/g, ' ').toUpperCase()}
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
    `;
  },

  renderHRTimeclock() {
    const employees = this.data.employees || [];
    const currentEmpId = this.selectedTimeclockEmpId || (employees[0] ? employees[0].id : '');
    const currentEmp = employees.find(e => e.id === currentEmpId) || employees[0] || {};
    const statusObj = this.data.timeclockStatus || { current_status: 'clocked_out', punches_today: [] };
    const curStatus = statusObj.current_status || statusObj.current_state || 'clocked_out';
    const punches = statusObj.punches_today || statusObj.events || [];

    const nowStr = new Date().toLocaleTimeString('en-US', { hour12: false });

    return `
      <div style="display:grid; grid-template-columns: 1fr 1.4fr; gap:24px;">
        <!-- Digital Punch Station Card -->
        <div class="timeclock-station">
          <div class="flex justify-between items-center">
            <div>
              <span class="text-xs uppercase tracking-widest text-slate-400 font-bold">Terminal Station</span>
              <h3 class="text-lg font-extrabold text-white">Digital Timeclock</h3>
            </div>
            <div class="flex items-center gap-2">
              <span class="w-2.5 h-2.5 rounded-full ${curStatus === 'clocked_in' ? 'bg-emerald-400 animate-pulse' : curStatus === 'on_break' ? 'bg-amber-400 animate-pulse' : 'bg-slate-400'}"></span>
              <span class="text-xs font-mono font-bold tracking-wider uppercase text-slate-300">
                ${curStatus.replace(/_/g, ' ')}
              </span>
            </div>
          </div>

          <div style="background:rgba(255,255,255,0.06); padding:16px; border-radius:12px; border:1px solid rgba(255,255,255,0.1);">
            <label class="text-[11px] font-bold text-slate-400 uppercase tracking-wider block mb-2">Select Active Employee</label>
            <select class="form-input text-sm" style="background:#0F172A; color:#FFFFFF; border-color:#334155;" onchange="app.setSelectedTimeclockEmployee(this.value)">
              ${employees.map(e => `
                <option value="${e.id}" ${e.id === currentEmpId ? 'selected' : ''}>${e.name} (${e.role || 'Staff'})</option>
              `).join('')}
            </select>
          </div>

          <div class="text-center py-2">
            <div class="timeclock-digital-face">${nowStr}</div>
            <div class="text-xs text-slate-400 font-medium mt-1">Official Server Time & Attendance Synchronization</div>
          </div>

          <div class="punch-grid">
            <button class="punch-btn punch-btn-in" ${curStatus !== 'clocked_out' ? 'disabled' : ''} onclick="app.recordPunch('clock_in')">
              <i data-lucide="log-in" class="w-5 h-5 mb-1"></i>
              Clock In
            </button>
            <button class="punch-btn punch-btn-break-start" ${curStatus !== 'clocked_in' ? 'disabled' : ''} onclick="app.recordPunch('break_start')">
              <i data-lucide="coffee" class="w-5 h-5 mb-1"></i>
              Start Break
            </button>
            <button class="punch-btn punch-btn-break-end" ${curStatus !== 'on_break' ? 'disabled' : ''} onclick="app.recordPunch('break_end')">
              <i data-lucide="play" class="w-5 h-5 mb-1"></i>
              End Break
            </button>
            <button class="punch-btn punch-btn-out" ${curStatus === 'clocked_out' ? 'disabled' : ''} onclick="app.recordPunch('clock_out')">
              <i data-lucide="log-out" class="w-5 h-5 mb-1"></i>
              Clock Out
            </button>
          </div>

          <div style="background:rgba(255,255,255,0.04); border-radius:10px; padding:12px 14px; display:flex; justify-content:space-between; align-items:center; font-size:12px; color:#94A3B8;">
            <span>Current Compensation:</span>
            <strong class="text-white font-mono">${currentEmp.salary_type === 'monthly' ? this.formatMoney((currentEmp.monthly_salary_cents || 0)/100) + '/mo' : this.formatMoney(currentEmp.hourly_rate || 0) + '/hr'}</strong>
          </div>
        </div>

        <!-- Today's Punch Activity Log -->
        <div class="table-container mockup-item">
          <div class="table-filter-bar">
            <div>
              <span class="font-bold text-[15px] text-slate-800">Punch Event Stream</span>
              <p class="text-xs text-slate-400 mt-0.5">Live time records for <strong>${currentEmp.name || 'Selected Employee'}</strong></p>
            </div>
            <button class="btn-press btn btn-secondary btn-sm" onclick="app.reloadTimeclockStatus().then(() => app.render())">
              <i data-lucide="refresh-cw" class="w-3.5 h-3.5"></i> Refresh
            </button>
          </div>
          <table class="data-table">
            <thead>
              <tr>
                <th>Event Type</th>
                <th>Timestamp</th>
                <th>Notes / Workstation</th>
                <th>Source IP</th>
              </tr>
            </thead>
            <tbody>
              ${punches.length === 0 ? `<tr><td colspan="4" class="text-center py-8 text-slate-400">No punches recorded today for this employee. Punch Clock In to begin shift.</td></tr>` :
                punches.map(p => `
                  <tr>
                    <td>
                      <span class="badge" style="background:${p.event_type === 'clock_in' ? '#ECFDF5' : p.event_type === 'clock_out' ? '#FEF2F2' : '#FFFBEB'}; color:${p.event_type === 'clock_in' ? '#047857' : p.event_type === 'clock_out' ? '#DC2626' : '#D97706'}; font-weight:700;">
                        ${p.event_type.replace(/_/g, ' ').toUpperCase()}
                      </span>
                    </td>
                    <td class="font-mono text-xs text-slate-600">${p.timestamp}</td>
                    <td class="text-slate-700">${p.notes || 'Terminal Punch'}</td>
                    <td class="font-mono text-xs text-slate-400">${p.ip_address || '127.0.0.1'}</td>
                  </tr>
                `).join('')}
            </tbody>
          </table>
        </div>
      </div>
    `;
  },

  renderHREmployees() {
    const emps = this.data.employees || [];

    return `
      <div class="table-container mockup-item" style="padding:24px; margin-bottom:24px;">
        <div class="flex justify-between items-center mb-6">
          <div>
            <h3 class="font-extrabold text-[18px] text-slate-900">Personnel & Compensation Registry</h3>
            <p class="text-xs text-slate-500 mt-0.5">Direct deposit banking, tax withholdings, and compensation structures.</p>
          </div>
          <div class="flex items-center gap-3">
            <input type="text" class="table-filter-input" style="width:240px;" placeholder="Search employee, title, department..." oninput="app.filterCards('employees-grid-container', 'emp-profile-card', this.value)">
            <button class="btn-press btn btn-primary" onclick="app.openNewEmployeeModal()">
              <i data-lucide="plus" class="w-4 h-4"></i> Onboard Member
            </button>
          </div>
        </div>

        <div id="employees-grid-container" style="display:grid; grid-template-columns:repeat(auto-fill, minmax(360px, 1fr)); gap:20px;">
          ${emps.length === 0 ? `<div class="text-slate-400 py-8 text-center col-span-full">No employee records registered. Click "+ Onboard Member" to add one.</div>` :
            emps.map(emp => {
              const compText = emp.salary_type === 'monthly'
                ? `${this.formatMoney((emp.monthly_salary_cents || 0) / 100)} / mo`
                : `${this.formatMoney(emp.hourly_rate || 0)} / hr`;
              const maskedAcct = emp.bank_account ? '••••' + emp.bank_account.slice(-4) : 'Not configured';

              return `
                <div class="emp-profile-card" style="background:#FFFFFF; border:1px solid var(--border-subtle); border-radius:14px; padding:20px; display:flex; flex-direction:column; justify-content:space-between; gap:16px; box-shadow:0 1px 3px rgba(0,0,0,0.04);">
                  <div>
                    <div class="flex justify-between items-start mb-2">
                      <div>
                        <h4 class="font-bold text-[16px] text-slate-900">${emp.name}</h4>
                        <div class="text-xs text-slate-500 font-medium">${emp.role || emp.job_title || 'Team Member'} • <span class="text-brand-blue font-semibold">${emp.department || 'General'}</span></div>
                      </div>
                      <span class="badge" style="background:#EFF6FF; color:#1D4ED8; font-size:11px; text-transform:uppercase;">
                        ${(emp.employment_type || 'full_time').replace(/_/g, ' ')}
                      </span>
                    </div>

                    <div style="background:#F8FAFC; border-radius:10px; padding:12px 14px; display:flex; flex-direction:column; gap:8px; margin-top:12px; font-size:12.5px;">
                      <div class="flex justify-between items-center">
                        <span class="text-slate-500">Compensation:</span>
                        <strong class="font-mono text-emerald-600 font-bold">${compText}</strong>
                      </div>
                      <div class="flex justify-between items-center">
                        <span class="text-slate-500">Tax ID:</span>
                        <span class="font-mono text-slate-700">${emp.tax_id || 'Pending'}</span>
                      </div>
                      <div class="flex justify-between items-center">
                        <span class="text-slate-500">Direct Deposit:</span>
                        <span class="font-mono text-slate-700">${emp.bank_name || 'Bank'} (${maskedAcct})</span>
                      </div>
                      <div class="flex justify-between items-center">
                        <span class="text-slate-500">Emergency Contact:</span>
                        <span class="text-slate-700">${emp.emergency_contact_name || '—'} ${emp.emergency_contact_phone ? '(' + emp.emergency_contact_phone + ')' : ''}</span>
                      </div>
                    </div>
                  </div>

                  <div class="flex justify-between items-center pt-2 border-t border-slate-100">
                    <span class="text-[11px] text-slate-400 font-mono">ID: ${emp.id}</span>
                    <button class="btn-press btn btn-secondary btn-sm" onclick="app.openEditEmployeeModal('${emp.id}')">
                      <i data-lucide="edit-3" class="w-3.5 h-3.5"></i> Edit Profile
                    </button>
                  </div>
                </div>
              `;
            }).join('')}
        </div>
      </div>
    `;
  },

  renderHRPayroll() {
    const preview = this.data.payrollPreview;
    const runs = this.data.payrollRuns || [];
    const payslips = this.data.payslips || [];
    const period = this.activePayrollPeriod || new Date().toISOString().slice(0, 7);

    // Compute preview totals or fallback to last run
    const grossCents = preview ? preview.total_gross_cents : (runs[0] ? runs[0].total_gross_cents : 0);
    const taxCents = preview ? preview.total_tax_cents : (runs[0] ? runs[0].total_tax_cents : 0);
    const deductionsCents = preview ? preview.total_deductions_cents : (runs[0] ? runs[0].total_deductions_cents : 0);
    const netCents = preview ? preview.total_net_cents : (runs[0] ? runs[0].total_net_cents : 0);
    const socialCents = Math.round(grossCents * 0.05);
    const benefitsCents = deductionsCents - socialCents > 0 ? deductionsCents - socialCents : Math.round(grossCents * 0.03);

    return `
      <!-- Period Selector & Disbursal Actions -->
      <div style="background:#FFFFFF; border:1px solid var(--border-subtle); border-radius:14px; padding:18px 24px; margin-bottom:24px; display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:16px;">
        <div class="flex items-center gap-4">
          <div class="flex flex-col">
            <label class="text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1">Pay Period (YYYY-MM)</label>
            <input type="month" id="payroll-period-input" class="form-input text-sm font-semibold" style="width:180px;" value="${period}" onchange="app.activePayrollPeriod = this.value">
          </div>
          <button class="btn-press btn btn-secondary mt-4" onclick="app.calculatePayrollPreview()">
            <i data-lucide="calculator" class="w-4 h-4"></i> Preview Payroll
          </button>
        </div>
        <div class="flex items-center gap-3">
          <button class="btn-press btn btn-primary" onclick="app.executePayrollRun()">
            <i data-lucide="send" class="w-4 h-4"></i> Disburse & Post to General Ledger
          </button>
        </div>
      </div>

      <!-- Real-time Payroll Summary KPI Grid -->
      <div class="payroll-summary-grid">
        <div class="payroll-stat-card">
          <span class="payroll-stat-label">Total Gross Wages</span>
          <span class="payroll-stat-val">${this.formatMoney(grossCents / 100)}</span>
          <span class="text-[11px] text-slate-400 font-medium">Debit Acc 5200</span>
        </div>
        <div class="payroll-stat-card">
          <span class="payroll-stat-label">Tax Withholding (12%)</span>
          <span class="payroll-stat-val text-amber-600">${this.formatMoney(taxCents / 100)}</span>
          <span class="text-[11px] text-slate-400 font-medium">Credit Acc 2150</span>
        </div>
        <div class="payroll-stat-card">
          <span class="payroll-stat-label">Social Contributions (5%)</span>
          <span class="payroll-stat-val text-blue-600">${this.formatMoney(socialCents / 100)}</span>
          <span class="text-[11px] text-slate-400 font-medium">Credit Acc 2150</span>
        </div>
        <div class="payroll-stat-card">
          <span class="payroll-stat-label">Benefits Deductions (3%)</span>
          <span class="payroll-stat-val text-purple-600">${this.formatMoney(benefitsCents / 100)}</span>
          <span class="text-[11px] text-slate-400 font-medium">Credit Acc 2150</span>
        </div>
        <div class="payroll-stat-card">
          <span class="payroll-stat-label">Net Disbursed Wages</span>
          <span class="payroll-stat-val payroll-stat-highlight">${this.formatMoney(netCents / 100)}</span>
          <span class="text-[11px] text-slate-400 font-medium">Credit Cash/Bank 1000</span>
        </div>
      </div>

      <!-- Itemized Payroll Run / Preview Calculation Table -->
      <div class="table-container mockup-item mb-6">
        <div class="table-filter-bar">
          <div>
            <span class="font-bold text-[15px] text-slate-800">Itemized Payroll Computation</span>
            <span class="text-xs text-slate-400 ml-2">Period: <strong>${period}</strong></span>
          </div>
          <span class="text-xs text-slate-500 font-mono">1.5x Overtime Multiplier Enforced</span>
        </div>
        <table class="data-table">
          <thead>
            <tr>
              <th>Employee</th>
              <th>Type</th>
              <th>Reg Hours</th>
              <th>OT Hours</th>
              <th>Gross Pay</th>
              <th>Tax (12%)</th>
              <th>Social (5%)</th>
              <th>Benefits (3%)</th>
              <th>Net Pay</th>
            </tr>
          </thead>
          <tbody>
            ${preview && preview.payslips && preview.payslips.length > 0 ?
              preview.payslips.map(p => `
                <tr>
                  <td><strong>${p.employee_name}</strong></td>
                  <td><span class="badge" style="background:#F1F5F9; color:#475569;">${p.salary_type || 'hourly'}</span></td>
                  <td class="font-mono text-xs">${(p.regular_hours || 0).toFixed(1)}h</td>
                  <td class="font-mono text-xs text-amber-600 font-bold">${(p.overtime_hours || 0).toFixed(1)}h</td>
                  <td class="font-mono text-xs font-bold">${this.formatMoney(p.gross_pay_cents / 100)}</td>
                  <td class="font-mono text-xs text-amber-600">-${this.formatMoney(p.tax_withheld_cents / 100)}</td>
                  <td class="font-mono text-xs text-blue-600">-${this.formatMoney(p.social_security_cents / 100)}</td>
                  <td class="font-mono text-xs text-purple-600">-${this.formatMoney(p.benefits_deduction_cents / 100)}</td>
                  <td class="font-mono text-xs font-bold text-emerald-600">${this.formatMoney(p.net_pay_cents / 100)}</td>
                </tr>
              `).join('') :
              `<tr><td colspan="9" class="text-center py-6 text-slate-400">Click "Preview Payroll" above to compute wages, overtime, and tax withholdings for ${period}.</td></tr>`
            }
          </tbody>
        </table>
      </div>

      <!-- Disbursed Payroll Runs & Payslip Archive -->
      <div style="display:grid; grid-template-columns: 1fr 1.2fr; gap:24px;">
        <div class="table-container mockup-item">
          <div class="table-filter-bar">
            <span class="font-bold text-[15px] text-slate-800">Payroll Run History</span>
          </div>
          <table class="data-table">
            <thead>
              <tr>
                <th>Period</th>
                <th>Disbursed</th>
                <th>Total Net</th>
                <th>Journal Entry</th>
              </tr>
            </thead>
            <tbody>
              ${runs.length === 0 ? `<tr><td colspan="4" class="text-center py-6 text-slate-400">No payroll runs posted yet.</td></tr>` :
                runs.map(r => `
                  <tr>
                    <td class="font-bold">${r.period}</td>
                    <td class="text-xs text-slate-500 font-mono">${(r.created_at || '').slice(0, 10)}</td>
                    <td class="font-mono text-xs font-bold text-emerald-600">${this.formatMoney(r.total_net_cents / 100)}</td>
                    <td><span class="badge" style="background:#EFF6FF; color:#1D4ED8; font-family:var(--font-mono); font-size:11px;">#${r.journal_entry_id ? r.journal_entry_id.slice(0, 8) : 'BALANCED'}</span></td>
                  </tr>
                `).join('')}
            </tbody>
          </table>
        </div>

        <div class="table-container mockup-item">
          <div class="table-filter-bar">
            <span class="font-bold text-[15px] text-slate-800">Employee Payslip Vault</span>
          </div>
          <table class="data-table">
            <thead>
              <tr>
                <th>Employee</th>
                <th>Period</th>
                <th>Net Pay</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              ${payslips.length === 0 ? `<tr><td colspan="4" class="text-center py-6 text-slate-400">No payslips generated. Disburse a payroll run to generate PDF payslips.</td></tr>` :
                payslips.map(ps => `
                  <tr>
                    <td><strong>${ps.employee_name || 'Staff'}</strong></td>
                    <td class="font-mono text-xs text-slate-500">${ps.period}</td>
                    <td class="font-mono text-xs font-bold text-emerald-600">${this.formatMoney(ps.net_pay_cents / 100)}</td>
                    <td>
                      <button class="btn-press btn btn-secondary btn-sm" onclick="app.downloadPayslipPDF('${ps.id}')">
                        <i data-lucide="download" class="w-3.5 h-3.5"></i> PDF
                      </button>
                    </td>
                  </tr>
                `).join('')}
            </tbody>
          </table>
        </div>
      </div>
    `;
  },

  renderHRLeaves() {
    const policies = this.data.leavePolicies || [];
    const requests = this.data.leaveRequests || [];

    return `
      <!-- Leave Policy Balance Chips -->
      <div class="pto-card-grid">
        ${policies.length === 0 ? `
          <div class="pto-card"><div><div class="text-xs text-slate-500 uppercase font-bold">Annual Vacation</div><div class="text-lg font-bold">20 Days</div></div><i data-lucide="sun" class="w-5 h-5 text-amber-500"></i></div>
          <div class="pto-card"><div><div class="text-xs text-slate-500 uppercase font-bold">Sick Leave</div><div class="text-lg font-bold">10 Days</div></div><i data-lucide="thermometer" class="w-5 h-5 text-red-500"></i></div>
          <div class="pto-card"><div><div class="text-xs text-slate-500 uppercase font-bold">Parental Leave</div><div class="text-lg font-bold">60 Days</div></div><i data-lucide="heart" class="w-5 h-5 text-purple-500"></i></div>
          <div class="pto-card"><div><div class="text-xs text-slate-500 uppercase font-bold">Unpaid Leave</div><div class="text-lg font-bold">30 Days</div></div><i data-lucide="clock" class="w-5 h-5 text-blue-500"></i></div>
        ` :
          policies.map(pol => `
            <div class="pto-card">
              <div>
                <div class="text-xs text-slate-500 uppercase font-bold tracking-wider">${pol.name}</div>
                <div class="text-lg font-bold text-slate-900 mt-0.5">${pol.days_allowed} Days / yr</div>
                <div class="text-[11px] text-slate-400 font-medium">${pol.paid ? '100% Paid Leave' : 'Unpaid Leave'}</div>
              </div>
              <span class="badge" style="background:#EFF6FF; color:#1D4ED8; text-transform:uppercase;">${pol.code}</span>
            </div>
          `).join('')}
      </div>

      <!-- Leave Requests Workflow Table -->
      <div class="table-container mockup-item">
        <div class="table-filter-bar">
          <div>
            <span class="font-bold text-[15px] text-slate-800">Leave & Time-Off Requests</span>
            <p class="text-xs text-slate-400 mt-0.5">Manager review and calendar synchronization</p>
          </div>
          <button class="btn-press btn btn-primary" onclick="app.openRequestLeaveModal()">
            <i data-lucide="calendar-plus" class="w-4 h-4"></i> Submit Leave Request
          </button>
        </div>
        <table class="data-table">
          <thead>
            <tr>
              <th>Employee</th>
              <th>Policy / Type</th>
              <th>Date Window</th>
              <th>Days</th>
              <th>Reason</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            ${requests.length === 0 ? `<tr><td colspan="7" class="text-center py-6 text-slate-400">No leave requests submitted. Click "Submit Leave Request" to create one.</td></tr>` :
              requests.map(req => `
                <tr>
                  <td><strong>${req.employee_name || 'Staff'}</strong></td>
                  <td><span class="badge" style="background:#F1F5F9; color:#475569; font-weight:700;">${req.policy_name || req.policy_id}</span></td>
                  <td class="font-mono text-xs text-slate-600">${req.start_date} — ${req.end_date}</td>
                  <td class="font-mono text-xs font-bold">${req.total_days}d</td>
                  <td class="text-slate-600 text-xs">${req.reason || '—'}</td>
                  <td>
                    <span class="badge" style="background:${req.status === 'approved' ? '#ECFDF5' : req.status === 'rejected' ? '#FEF2F2' : '#FFFBEB'}; color:${req.status === 'approved' ? '#047857' : req.status === 'rejected' ? '#DC2626' : '#D97706'}; border:1px solid ${req.status === 'approved' ? '#A7F3D0' : req.status === 'rejected' ? '#FECACA' : '#FDE68A'};">
                      ${req.status.toUpperCase()}
                    </span>
                  </td>
                  <td>
                    ${req.status === 'pending' ? `
                      <div class="flex items-center gap-1.5">
                        <button class="btn-press btn btn-sm" style="background:#10B981; color:#FFFFFF; padding:4px 8px; font-size:11px;" onclick="app.reviewLeaveRequest('${req.id}', 'approved')">
                          Approve
                        </button>
                        <button class="btn-press btn btn-sm" style="background:#EF4444; color:#FFFFFF; padding:4px 8px; font-size:11px;" onclick="app.reviewLeaveRequest('${req.id}', 'rejected')">
                          Reject
                        </button>
                      </div>
                    ` : `<span class="text-xs text-slate-400 font-mono">Reviewed</span>`}
                  </td>
                </tr>
              `).join('')}
          </tbody>
        </table>
      </div>
    `;
  },

  renderHRDocuments() {
    const docs = this.data.hrDocuments || [];

    return `
      <div class="table-container mockup-item" style="padding:24px;">
        <div class="flex justify-between items-center mb-6">
          <div>
            <div class="flex items-center gap-2 mb-1">
              <span class="badge" style="background:#FEF2F2; color:#B91C1C; font-weight:800; font-size:10px; letter-spacing:0.5px;">ROLE-GATED (MANAGE_HR)</span>
              <h3 class="font-extrabold text-[18px] text-slate-900">HR Compliance & Document Vault</h3>
            </div>
            <p class="text-xs text-slate-500">Secure storage for contracts, NDAs, identification verifications, and performance reviews.</p>
          </div>
          <button class="btn-press btn btn-primary" onclick="app.openUploadDocModal()">
            <i data-lucide="upload-cloud" class="w-4 h-4"></i> Upload Document
          </button>
        </div>

        <div style="display:grid; grid-template-columns:repeat(auto-fill, minmax(320px, 1fr)); gap:16px;">
          ${docs.length === 0 ? `<div class="text-slate-400 py-8 text-center col-span-full">No documents stored in the vault yet. Click "Upload Document" to archive compliance records.</div>` :
            docs.map(doc => `
              <div style="background:#F8FAFC; border:1px solid var(--border-subtle); border-radius:12px; padding:16px; display:flex; flex-direction:column; justify-content:space-between; gap:12px;">
                <div class="flex items-start justify-between">
                  <div class="flex items-center gap-3">
                    <div style="width:40px; height:40px; border-radius:10px; background:#EFF6FF; color:#2563EB; display:flex; align-items:center; justify-content:center;">
                      <i data-lucide="file-text" class="w-5 h-5"></i>
                    </div>
                    <div>
                      <h4 class="font-bold text-sm text-slate-900">${doc.title}</h4>
                      <span class="text-xs text-slate-500 font-medium">${doc.employee_name || 'Organization'}</span>
                    </div>
                  </div>
                  <span class="badge" style="background:#FFFFFF; border:1px solid #E2E8F0; color:#334155; font-size:10.5px; text-transform:uppercase;">
                    ${doc.doc_type || 'Contract'}
                  </span>
                </div>

                <div class="flex justify-between items-center text-xs text-slate-400 font-mono pt-2 border-t border-slate-200/60">
                  <span>${doc.file_name || 'document.pdf'}</span>
                  <span>${((doc.file_size_bytes || 102400) / 1024).toFixed(0)} KB</span>
                </div>
              </div>
            `).join('')}
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

  // ---------------- Module 11: Financial Intelligence, Statements & Exports ----------------
  async ensureReportData(subTab) {
    try {
      if (subTab === 'kpis' && !this.data.reportsKPIs) {
        const res = await fetch('/api/reports/executive-kpis');
        if (res.ok) this.data.reportsKPIs = await res.json();
      } else if (subTab === 'pnl' && !this.data.reportsPnL) {
        await this.reloadPnLReport();
      } else if (subTab === 'balance-sheet' && !this.data.reportsBalanceSheet) {
        await this.reloadBalanceSheetReport();
      } else if (subTab === 'cash-flow' && !this.data.reportsCashFlow) {
        await this.reloadCashFlowReport();
      } else if (subTab === 'aging') {
        await this.reloadAgingReport();
      } else if (subTab === 'inventory') {
        await this.reloadInventoryReports();
      } else if (subTab === 'sales' && !this.data.reportsSalesVelocity) {
        const res = await fetch('/api/reports/sales-velocity');
        if (res.ok) this.data.reportsSalesVelocity = await res.json();
      }
    } catch (err) {
      console.warn('Failed to ensure report data:', err);
    }
  },

  async reloadPnLReport() {
    const f = this.reportsFilter;
    try {
      const res = await fetch(`/api/reports/pnl?start_date=${encodeURIComponent(f.pnlStart)}&end_date=${encodeURIComponent(f.pnlEnd)}`);
      if (res.ok) {
        this.data.reportsPnL = await res.json();
        if (this.activeTab === 'reports' && this.activeReportsSubTab === 'pnl') this.render();
      }
    } catch (e) { console.error('Error fetching P&L:', e); }
  },

  async reloadBalanceSheetReport() {
    const f = this.reportsFilter;
    try {
      const res = await fetch(`/api/reports/balance-sheet?as_of_date=${encodeURIComponent(f.balanceSheetAsOf)}`);
      if (res.ok) {
        this.data.reportsBalanceSheet = await res.json();
        if (this.activeTab === 'reports' && this.activeReportsSubTab === 'balance-sheet') this.render();
      }
    } catch (e) { console.error('Error fetching Balance Sheet:', e); }
  },

  async reloadCashFlowReport() {
    const f = this.reportsFilter;
    try {
      const res = await fetch(`/api/reports/cash-flow?start_date=${encodeURIComponent(f.cashFlowStart)}&end_date=${encodeURIComponent(f.cashFlowEnd)}`);
      if (res.ok) {
        this.data.reportsCashFlow = await res.json();
        if (this.activeTab === 'reports' && this.activeReportsSubTab === 'cash-flow') this.render();
      }
    } catch (e) { console.error('Error fetching Cash Flow:', e); }
  },

  async reloadAgingReport() {
    const f = this.reportsFilter;
    const ep = f.agingType === 'ap' ? '/api/reports/aging-ap' : '/api/reports/aging-ar';
    try {
      const res = await fetch(`${ep}?as_of_date=${encodeURIComponent(f.agingAsOf)}`);
      if (res.ok) {
        if (f.agingType === 'ap') {
          this.data.reportsAgingAP = await res.json();
        } else {
          this.data.reportsAgingAR = await res.json();
        }
        if (this.activeTab === 'reports' && this.activeReportsSubTab === 'aging') this.render();
      }
    } catch (e) { console.error('Error fetching Aging report:', e); }
  },

  async reloadInventoryReports() {
    try {
      const [valRes, turnRes] = await Promise.all([
        fetch('/api/reports/inventory-valuation'),
        fetch('/api/reports/inventory-turnover')
      ]);
      if (valRes.ok) this.data.reportsInventoryValuation = await valRes.json();
      if (turnRes.ok) this.data.reportsInventoryTurnover = await turnRes.json();
      if (this.activeTab === 'reports' && this.activeReportsSubTab === 'inventory') this.render();
    } catch (e) { console.error('Error fetching inventory reports:', e); }
  },

  async reloadExecutiveKPIs() {
    try {
      const res = await fetch('/api/reports/executive-kpis');
      if (res.ok) {
        this.data.reportsKPIs = await res.json();
        if (this.activeTab === 'reports' && this.activeReportsSubTab === 'kpis') this.render();
      }
    } catch (e) { console.error('Error fetching KPIs:', e); }
  },

  async reloadSalesVelocity() {
    try {
      const res = await fetch('/api/reports/sales-velocity');
      if (res.ok) {
        this.data.reportsSalesVelocity = await res.json();
        if (this.activeTab === 'reports' && this.activeReportsSubTab === 'sales') this.render();
      }
    } catch (e) { console.error('Error fetching Sales Velocity:', e); }
  },

  triggerReportExport(entity, format, startDate = null, endDate = null, asOfDate = null) {
    let url = `/api/reports/export?entity=${encodeURIComponent(entity)}&format=${encodeURIComponent(format)}`;
    if (startDate) url += `&start_date=${encodeURIComponent(startDate)}`;
    if (endDate) url += `&end_date=${encodeURIComponent(endDate)}`;
    if (asOfDate) url += `&as_of_date=${encodeURIComponent(asOfDate)}`;
    
    const a = document.createElement('a');
    a.href = url;
    a.download = `${entity}_${new Date().toISOString().slice(0,10)}.${format}`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    this.showToast(`Export initiated: ${entity.toUpperCase()} (${format.toUpperCase()})`);
  },

  renderReports() {
    const subTab = this.activeReportsSubTab || 'kpis';

    let content = '';
    switch (subTab) {
      case 'kpis':
        content = this.renderReportsKPIs();
        break;
      case 'pnl':
        content = this.renderReportsPnL();
        break;
      case 'balance-sheet':
        content = this.renderReportsBalanceSheet();
        break;
      case 'cash-flow':
        content = this.renderReportsCashFlow();
        break;
      case 'aging':
        content = this.renderReportsAging();
        break;
      case 'inventory':
        content = this.renderReportsInventory();
        break;
      case 'sales':
        content = this.renderReportsSales();
        break;
      case 'export':
        content = this.renderReportsExport();
        break;
      default:
        content = this.renderReportsKPIs();
    }

    return `
      <div class="page-header">
        <div>
          <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-blue-100 text-brand-blue text-[11px] font-bold uppercase tracking-widest mb-3">
            <i data-lucide="bar-chart-3" class="w-3.5 h-3.5"></i> Financial Intelligence & Exports
          </div>
          <h1 class="page-title">Intelligence, Statements & Exports</h1>
          <p class="page-sub">Audited real-time financial statements, aging ledgers, operational turnover, and vector document exports.</p>
        </div>
        <div class="page-actions">
          <button class="btn-press btn btn-secondary" onclick="app.setReportsSubTab('export')">
            <i data-lucide="download" class="w-4 h-4 text-brand-blue"></i>
            Export Hub
          </button>
          <button class="btn-press btn btn-primary" onclick="app.ensureReportData(app.activeReportsSubTab); app.showToast('Report data refreshed');">
            <i data-lucide="refresh-cw" class="w-4 h-4"></i>
            Refresh Intelligence
          </button>
        </div>
      </div>

      <!-- Reports Sub-navigation Bar -->
      <div class="subnav-bar">
        <button class="subnav-pill ${subTab === 'kpis' ? 'active' : ''}" onclick="app.setReportsSubTab('kpis')">
          <i data-lucide="activity" class="w-4 h-4"></i> Executive KPIs
        </button>
        <button class="subnav-pill ${subTab === 'pnl' ? 'active' : ''}" onclick="app.setReportsSubTab('pnl')">
          <i data-lucide="trending-up" class="w-4 h-4"></i> Profit & Loss (P&L)
        </button>
        <button class="subnav-pill ${subTab === 'balance-sheet' ? 'active' : ''}" onclick="app.setReportsSubTab('balance-sheet')">
          <i data-lucide="scale" class="w-4 h-4"></i> Balance Sheet
        </button>
        <button class="subnav-pill ${subTab === 'cash-flow' ? 'active' : ''}" onclick="app.setReportsSubTab('cash-flow')">
          <i data-lucide="arrow-left-right" class="w-4 h-4"></i> Cash Flow
        </button>
        <button class="subnav-pill ${subTab === 'aging' ? 'active' : ''}" onclick="app.setReportsSubTab('aging')">
          <i data-lucide="clock" class="w-4 h-4"></i> Aged A/R & A/P
        </button>
        <button class="subnav-pill ${subTab === 'inventory' ? 'active' : ''}" onclick="app.setReportsSubTab('inventory')">
          <i data-lucide="boxes" class="w-4 h-4"></i> Inventory Turnover
        </button>
        <button class="subnav-pill ${subTab === 'sales' ? 'active' : ''}" onclick="app.setReportsSubTab('sales')">
          <i data-lucide="zap" class="w-4 h-4"></i> Sales Velocity
        </button>
        <button class="subnav-pill ${subTab === 'export' ? 'active' : ''}" onclick="app.setReportsSubTab('export')">
          <i data-lucide="file-output" class="w-4 h-4"></i> Document Exports
        </button>
      </div>

      <div class="reports-content-wrapper">
        ${content}
      </div>
    `;
  },

  renderReportsKPIs() {
    const k = this.data.reportsKPIs || {};
    const mrr = k.mrr !== undefined ? k.mrr : (k.mrr_cents || 0) / 100;
    const operatingCash = k.operating_cash !== undefined ? k.operating_cash : (k.operating_cash_cents || 0) / 100;
    const commitments = k.open_purchase_commitments !== undefined ? k.open_purchase_commitments : (k.open_purchase_commitments_cents || 0) / 100;
    const ytdRev = k.total_revenue_ytd !== undefined ? k.total_revenue_ytd : (k.ytd_revenue_cents || 0) / 100;
    const ytdNet = k.net_profit_ytd !== undefined ? k.net_profit_ytd : (k.ytd_net_profit_cents || 0) / 100;
    const runwayMonths = k.cash_runway_months || 0;
    const grossMarginPct = k.gross_margin_pct || 0;
    const pendingApprs = k.pending_approvals_count || 0;

    const runwayBadge = runwayMonths >= 12
      ? '<span class="status-badge status-approved">12+ Months • Extended Runway</span>'
      : runwayMonths >= 6
      ? '<span class="status-badge status-delivered">Healthy Runway (>6m)</span>'
      : '<span class="status-badge status-rejected">Critical Runway (<6m)</span>';

    return `
      <!-- Executive Top KPI Grid -->
      <div class="stats-grid mb-6">
        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-2">
            <div class="stat-title">Monthly Recurring Revenue</div>
            <div class="w-10 h-10 bg-blue-50 rounded-xl flex items-center justify-center">
              <i data-lucide="repeat" class="w-5 h-5 text-brand-blue"></i>
            </div>
          </div>
          <div class="stat-value">${this.formatMoney(mrr)}</div>
          <div class="stat-trend trend-up">MRR Run-rate</div>
        </div>

        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-2">
            <div class="stat-title">Gross Margin %</div>
            <div class="w-10 h-10 bg-emerald-50 rounded-xl flex items-center justify-center">
              <i data-lucide="percent" class="w-5 h-5 text-emerald-500"></i>
            </div>
          </div>
          <div class="stat-value" style="color:#059669;">${grossMarginPct.toFixed(1)}%</div>
          <div class="stat-trend trend-up">Revenue vs COGS efficiency</div>
        </div>

        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-2">
            <div class="stat-title">Operating Cash</div>
            <div class="w-10 h-10 bg-indigo-50 rounded-xl flex items-center justify-center">
              <i data-lucide="wallet" class="w-5 h-5 text-indigo-600"></i>
            </div>
          </div>
          <div class="stat-value">${this.formatMoney(operatingCash)}</div>
          <div class="stat-trend font-medium text-slate-500">Account 1000 Reserves</div>
        </div>

        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-2">
            <div class="stat-title">Cash Runway</div>
            <div class="w-10 h-10 bg-amber-50 rounded-xl flex items-center justify-center">
              <i data-lucide="hourglass" class="w-5 h-5 text-amber-500"></i>
            </div>
          </div>
          <div class="stat-value">${runwayMonths.toFixed(1)} <span class="text-xs font-semibold text-slate-400">Months</span></div>
          <div class="stat-trend">${runwayBadge}</div>
        </div>
      </div>

      <!-- Secondary KPI Row -->
      <div class="stats-grid mb-6">
        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-2">
            <div class="stat-title">YTD Gross Revenue</div>
            <div class="w-10 h-10 bg-purple-50 rounded-xl flex items-center justify-center">
              <i data-lucide="trending-up" class="w-5 h-5 text-purple-600"></i>
            </div>
          </div>
          <div class="stat-value">${this.formatMoney(ytdRev)}</div>
          <div class="stat-trend trend-up">Year-to-date booked</div>
        </div>

        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-2">
            <div class="stat-title">YTD Net Profit</div>
            <div class="w-10 h-10 bg-emerald-50 rounded-xl flex items-center justify-center">
              <i data-lucide="award" class="w-5 h-5 text-emerald-600"></i>
            </div>
          </div>
          <div class="stat-value" style="color:#059669;">${this.formatMoney(ytdNet)}</div>
          <div class="stat-trend trend-up">Net margin bottom-line</div>
        </div>

        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-2">
            <div class="stat-title">Open Commitments</div>
            <div class="w-10 h-10 bg-orange-50 rounded-xl flex items-center justify-center">
              <i data-lucide="truck" class="w-5 h-5 text-orange-500"></i>
            </div>
          </div>
          <div class="stat-value">${this.formatMoney(commitments)}</div>
          <div class="stat-trend text-slate-500">Issued & Ordered POs</div>
        </div>

        <div class="stat-card mockup-item">
          <div class="flex justify-between items-start mb-2">
            <div class="stat-title">Pending Governance</div>
            <div class="w-10 h-10 bg-rose-50 rounded-xl flex items-center justify-center">
              <i data-lucide="shield-alert" class="w-5 h-5 text-rose-500"></i>
            </div>
          </div>
          <div class="stat-value" style="color:${pendingApprs > 0 ? '#E11D48' : '#059669'};">${pendingApprs} <span class="text-xs font-semibold text-slate-400">Actions</span></div>
          <div class="stat-trend text-slate-500 font-medium">Awaiting Dual-Actor Sign-off</div>
        </div>
      </div>

      <!-- Strategic Directives Card -->
      <div class="financial-statement-card">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h3 class="text-base font-bold text-brand-textMain">Executive Briefing & Fast Actions</h3>
            <p class="text-xs text-slate-400">Direct shortcuts to audit-grade statements and operational actions.</p>
          </div>
          <div class="flex items-center gap-2">
            <button class="btn btn-secondary text-xs" onclick="app.setReportsSubTab('pnl')">View Full P&L</button>
            <button class="btn btn-secondary text-xs" onclick="app.setReportsSubTab('balance-sheet')">View Balance Sheet</button>
            <button class="btn btn-secondary text-xs" onclick="app.setReportsSubTab('aging')">Review Aging</button>
          </div>
        </div>
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4 pt-2">
          <div class="p-4 rounded-xl bg-slate-50 border border-slate-200">
            <div class="text-xs font-bold text-slate-500 uppercase tracking-wider mb-1">Liquidity & Runway</div>
            <div class="text-sm text-slate-700">Operating balance stands at <strong>${this.formatMoney(k.operating_cash_cents / 100)}</strong> providing <strong>${k.cash_runway_months.toFixed(1)} months</strong> of continuous runway under active expenditure rates.</div>
          </div>
          <div class="p-4 rounded-xl bg-slate-50 border border-slate-200">
            <div class="text-xs font-bold text-slate-500 uppercase tracking-wider mb-1">Profitability & Margin</div>
            <div class="text-sm text-slate-700">Gross margin maintained at <strong>${k.gross_margin_pct.toFixed(1)}%</strong> with YTD bottom-line net profit of <strong>${this.formatMoney(k.ytd_net_profit_cents / 100)}</strong>.</div>
          </div>
          <div class="p-4 rounded-xl bg-slate-50 border border-slate-200">
            <div class="text-xs font-bold text-slate-500 uppercase tracking-wider mb-1">Procurement & Outlays</div>
            <div class="text-sm text-slate-700">Active purchase order commitments total <strong>${this.formatMoney(k.open_purchase_commitments_cents / 100)}</strong> across pending vendor deliveries.</div>
          </div>
        </div>
      </div>
    `;
  },

  renderReportsPnL() {
    const pnl = this.data.reportsPnL || {
      start_date: this.reportsFilter.pnlStart,
      end_date: this.reportsFilter.pnlEnd,
      revenue_cents: 0,
      cogs_cents: 0,
      gross_profit_cents: 0,
      gross_margin_pct: 0,
      opex_cents: 0,
      net_profit_cents: 0,
      items: []
    };

    const revItems = (pnl.items || []).filter(i => i.category === 'revenue');
    const cogsItems = (pnl.items || []).filter(i => i.category === 'cogs');
    const opexItems = (pnl.items || []).filter(i => i.category === 'opex');

    return `
      <!-- Filter & Action Toolbar -->
      <div class="reports-toolbar">
        <div class="reports-date-group">
          <span>Period:</span>
          <input type="date" class="reports-date-input" value="${this.reportsFilter.pnlStart}" onchange="app.setReportsFilter('pnlStart', this.value)">
          <span>to</span>
          <input type="date" class="reports-date-input" value="${this.reportsFilter.pnlEnd}" onchange="app.setReportsFilter('pnlEnd', this.value)">
          <button class="btn-press btn btn-primary text-xs py-1.5 px-3" onclick="app.reloadPnLReport()">
            <i data-lucide="play" class="w-3.5 h-3.5"></i> Run Statement
          </button>
        </div>
        <div class="flex items-center gap-2">
          <button class="btn-press btn btn-secondary text-xs" onclick="app.triggerReportExport('pnl', 'csv', app.reportsFilter.pnlStart, app.reportsFilter.pnlEnd)">
            <i data-lucide="file-spreadsheet" class="w-3.5 h-3.5 text-emerald-600"></i> Export CSV
          </button>
          <button class="btn-press btn btn-secondary text-xs" onclick="app.triggerReportExport('pnl', 'pdf', app.reportsFilter.pnlStart, app.reportsFilter.pnlEnd)">
            <i data-lucide="file-text" class="w-3.5 h-3.5 text-rose-600"></i> Vector PDF
          </button>
        </div>
      </div>

      <!-- P&L Metric Highlights -->
      <div class="stats-grid mb-6">
        <div class="stat-card mockup-item">
          <div class="stat-title">Operating Revenue</div>
          <div class="stat-value">${this.formatMoney(pnl.revenue_cents / 100)}</div>
          <div class="stat-trend trend-up">Invoiced & Earned</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Cost of Goods Sold</div>
          <div class="stat-value">${this.formatMoney(pnl.cogs_cents / 100)}</div>
          <div class="stat-trend text-slate-500 font-medium">Direct Production / Purchase</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Gross Profit</div>
          <div class="stat-value" style="color:#059669;">${this.formatMoney(pnl.gross_profit_cents / 100)}</div>
          <div class="stat-trend trend-up">Gross Margin: ${pnl.gross_margin_pct.toFixed(1)}%</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Operating Expenses</div>
          <div class="stat-value">${this.formatMoney(pnl.opex_cents / 100)}</div>
          <div class="stat-trend text-slate-500 font-medium">Payroll & Overhead</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Net Operating Profit</div>
          <div class="stat-value" style="color:${pnl.net_profit_cents >= 0 ? '#059669' : '#DC2626'};">${this.formatMoney(pnl.net_profit_cents / 100)}</div>
          <div class="stat-trend ${pnl.net_profit_cents >= 0 ? 'trend-up' : 'text-rose-500'}">${pnl.net_profit_cents >= 0 ? 'Net Income' : 'Net Deficit'}</div>
        </div>
      </div>

      <!-- Financial Statement Table -->
      <div class="table-container mockup-item">
        <table class="data-table">
          <thead>
            <tr>
              <th style="width:140px;">Account Code</th>
              <th>Account Description</th>
              <th style="width:160px;">Classification</th>
              <th style="width:180px; text-align:right;">Period Amount</th>
            </tr>
          </thead>
          <tbody>
            <!-- REVENUE SECTION -->
            <tr class="statement-row-category">
              <td colspan="4">I. Operating Revenue</td>
            </tr>
            ${revItems.length > 0 ? revItems.map(item => `
              <tr>
                <td class="mono-code">${item.account_code}</td>
                <td><strong>${item.account_name}</strong></td>
                <td><span class="actor-pill actor-agent">REVENUE</span></td>
                <td style="text-align:right; font-family:var(--font-mono); font-weight:700; color:#059669;">${this.formatMoney(item.amount_cents / 100)}</td>
              </tr>
            `).join('') : `<tr><td colspan="4" class="text-center py-3 text-slate-400">No revenue entries recorded for selected period</td></tr>`}
            <tr class="statement-row-total">
              <td colspan="3" style="text-align:right; padding-right:16px;">Total Operating Revenue:</td>
              <td style="text-align:right; font-family:var(--font-mono); font-weight:800; color:#059669;">${this.formatMoney(pnl.revenue_cents / 100)}</td>
            </tr>

            <!-- COGS SECTION -->
            <tr class="statement-row-category">
              <td colspan="4">II. Cost of Goods Sold (COGS)</td>
            </tr>
            ${cogsItems.length > 0 ? cogsItems.map(item => `
              <tr>
                <td class="mono-code">${item.account_code}</td>
                <td><strong>${item.account_name}</strong></td>
                <td><span class="actor-pill actor-human">COGS</span></td>
                <td style="text-align:right; font-family:var(--font-mono); font-weight:700; color:#E11D48;">${this.formatMoney(item.amount_cents / 100)}</td>
              </tr>
            `).join('') : `<tr><td colspan="4" class="text-center py-3 text-slate-400">No direct inventory/production COGS entries</td></tr>`}
            <tr class="statement-row-total">
              <td colspan="3" style="text-align:right; padding-right:16px;">Gross Profit (Revenue - COGS):</td>
              <td style="text-align:right; font-family:var(--font-mono); font-weight:800; color:#059669;">${this.formatMoney(pnl.gross_profit_cents / 100)}</td>
            </tr>

            <!-- OPEX SECTION -->
            <tr class="statement-row-category">
              <td colspan="4">III. Operating Expenses (OpEx)</td>
            </tr>
            ${opexItems.length > 0 ? opexItems.map(item => `
              <tr>
                <td class="mono-code">${item.account_code}</td>
                <td><strong>${item.account_name}</strong></td>
                <td><span class="actor-pill actor-human">OPEX</span></td>
                <td style="text-align:right; font-family:var(--font-mono); font-weight:700; color:#475569;">${this.formatMoney(item.amount_cents / 100)}</td>
              </tr>
            `).join('') : `<tr><td colspan="4" class="text-center py-3 text-slate-400">No overhead expenses recorded</td></tr>`}
            <tr class="statement-row-total">
              <td colspan="3" style="text-align:right; padding-right:16px;">Total Operating Expenses:</td>
              <td style="text-align:right; font-family:var(--font-mono); font-weight:800; color:#475569;">${this.formatMoney(pnl.opex_cents / 100)}</td>
            </tr>

            <!-- NET INCOME -->
            <tr class="statement-row-total" style="background:rgba(16, 185, 129, 0.08);">
              <td colspan="3" style="text-align:right; font-size:16px; padding-right:16px; color:#059669;">NET PROFIT / (LOSS):</td>
              <td style="text-align:right; font-family:var(--font-mono); font-weight:900; font-size:16px; color:${pnl.net_profit_cents >= 0 ? '#059669' : '#DC2626'};">${this.formatMoney(pnl.net_profit_cents / 100)}</td>
            </tr>
          </tbody>
        </table>
      </div>
    `;
  },

  renderReportsBalanceSheet() {
    const bs = this.data.reportsBalanceSheet || {
      as_of_date: this.reportsFilter.balanceSheetAsOf,
      total_assets_cents: 0,
      total_liabilities_cents: 0,
      total_equity_cents: 0,
      is_balanced: true,
      assets: [],
      liabilities: [],
      equity: []
    };

    const isBalanced = bs.is_balanced;
    const balanceDiff = bs.total_assets_cents - (bs.total_liabilities_cents + bs.total_equity_cents);

    return `
      <div class="reports-toolbar">
        <div class="reports-date-group">
          <span>Statement As Of:</span>
          <input type="date" class="reports-date-input" value="${this.reportsFilter.balanceSheetAsOf}" onchange="app.setReportsFilter('balanceSheetAsOf', this.value)">
          <button class="btn-press btn btn-primary text-xs py-1.5 px-3" onclick="app.reloadBalanceSheetReport()">
            <i data-lucide="play" class="w-3.5 h-3.5"></i> Run Balance Sheet
          </button>
        </div>
        <div class="flex items-center gap-3">
          ${isBalanced ? `
            <span class="balance-check-pill balanced" title="Assets = Liabilities + Equity strictly satisfied">
              <i data-lucide="check-circle" class="w-4 h-4"></i> Balanced Equation Verified
            </span>
          ` : `
            <span class="balance-check-pill unbalanced" title="Variance detected: ${this.formatMoney(balanceDiff / 100)}">
              <i data-lucide="alert-triangle" class="w-4 h-4"></i> Equation Out of Balance (${this.formatMoney(balanceDiff / 100)})
            </span>
          `}
          <button class="btn-press btn btn-secondary text-xs" onclick="app.triggerReportExport('balance-sheet', 'csv', null, null, app.reportsFilter.balanceSheetAsOf)">
            <i data-lucide="file-spreadsheet" class="w-3.5 h-3.5 text-emerald-600"></i> Export CSV
          </button>
          <button class="btn-press btn btn-secondary text-xs" onclick="app.triggerReportExport('balance-sheet', 'pdf', null, null, app.reportsFilter.balanceSheetAsOf)">
            <i data-lucide="file-text" class="w-3.5 h-3.5 text-rose-600"></i> Vector PDF
          </button>
        </div>
      </div>

      <!-- High-Level Overview Grid -->
      <div class="stats-grid mb-6">
        <div class="stat-card mockup-item">
          <div class="stat-title">Total Assets (Debit)</div>
          <div class="stat-value" style="color:#2563EB;">${this.formatMoney(bs.total_assets_cents / 100)}</div>
          <div class="stat-trend trend-up">Cash, Receivables & Stock</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Total Liabilities (Credit)</div>
          <div class="stat-value" style="color:#E11D48;">${this.formatMoney(bs.total_liabilities_cents / 100)}</div>
          <div class="stat-trend text-slate-500 font-medium">Payables & Obligations</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Total Equity (Credit)</div>
          <div class="stat-value" style="color:#059669;">${this.formatMoney(bs.total_equity_cents / 100)}</div>
          <div class="stat-trend trend-up">Capital & Current Earnings</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Liabilities + Equity</div>
          <div class="stat-value">${this.formatMoney((bs.total_liabilities_cents + bs.total_equity_cents) / 100)}</div>
          <div class="stat-trend ${isBalanced ? 'trend-up' : 'text-rose-500'}">${isBalanced ? '✓ Equation Exact (Zero Variance)' : 'Variance: ' + this.formatMoney(balanceDiff / 100)}</div>
        </div>
      </div>

      <!-- 3-Column Detailed Ledger Layout -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <!-- ASSETS -->
        <div class="financial-statement-card">
          <div class="flex items-center justify-between pb-3 border-b border-slate-200 mb-3">
            <h3 class="font-extrabold text-sm text-brand-textMain uppercase tracking-wider flex items-center gap-2">
              <i data-lucide="landmark" class="w-4 h-4 text-brand-blue"></i> Assets (1000s)
            </h3>
            <span class="mono-code font-bold text-blue-600">${this.formatMoney(bs.total_assets_cents / 100)}</span>
          </div>
          <table class="w-full text-xs">
            <thead>
              <tr class="text-slate-400 border-b border-slate-100">
                <th class="py-2 text-left">Account</th>
                <th class="py-2 text-right">Balance</th>
              </tr>
            </thead>
            <tbody>
              ${(bs.assets || []).map(a => `
                <tr class="border-b border-slate-50 hover:bg-slate-50/50">
                  <td class="py-2.5">
                    <span class="mono-code text-[11px] text-slate-400 mr-1.5">${a.code}</span>
                    <strong class="text-slate-700">${a.name}</strong>
                  </td>
                  <td class="py-2.5 text-right font-mono font-semibold text-slate-800">${this.formatMoney(a.balance_cents / 100)}</td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>

        <!-- LIABILITIES -->
        <div class="financial-statement-card">
          <div class="flex items-center justify-between pb-3 border-b border-slate-200 mb-3">
            <h3 class="font-extrabold text-sm text-brand-textMain uppercase tracking-wider flex items-center gap-2">
              <i data-lucide="credit-card" class="w-4 h-4 text-rose-500"></i> Liabilities (2000s)
            </h3>
            <span class="mono-code font-bold text-rose-600">${this.formatMoney(bs.total_liabilities_cents / 100)}</span>
          </div>
          <table class="w-full text-xs">
            <thead>
              <tr class="text-slate-400 border-b border-slate-100">
                <th class="py-2 text-left">Account</th>
                <th class="py-2 text-right">Balance</th>
              </tr>
            </thead>
            <tbody>
              ${(bs.liabilities || []).map(l => `
                <tr class="border-b border-slate-50 hover:bg-slate-50/50">
                  <td class="py-2.5">
                    <span class="mono-code text-[11px] text-slate-400 mr-1.5">${l.code}</span>
                    <strong class="text-slate-700">${l.name}</strong>
                  </td>
                  <td class="py-2.5 text-right font-mono font-semibold text-slate-800">${this.formatMoney(l.balance_cents / 100)}</td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>

        <!-- EQUITY -->
        <div class="financial-statement-card">
          <div class="flex items-center justify-between pb-3 border-b border-slate-200 mb-3">
            <h3 class="font-extrabold text-sm text-brand-textMain uppercase tracking-wider flex items-center gap-2">
              <i data-lucide="pie-chart" class="w-4 h-4 text-emerald-500"></i> Equity (3000s)
            </h3>
            <span class="mono-code font-bold text-emerald-600">${this.formatMoney(bs.total_equity_cents / 100)}</span>
          </div>
          <table class="w-full text-xs">
            <thead>
              <tr class="text-slate-400 border-b border-slate-100">
                <th class="py-2 text-left">Account</th>
                <th class="py-2 text-right">Balance</th>
              </tr>
            </thead>
            <tbody>
              ${(bs.equity || []).map(e => `
                <tr class="border-b border-slate-50 hover:bg-slate-50/50">
                  <td class="py-2.5">
                    <span class="mono-code text-[11px] text-slate-400 mr-1.5">${e.code}</span>
                    <strong class="text-slate-700">${e.name}</strong>
                  </td>
                  <td class="py-2.5 text-right font-mono font-semibold text-slate-800">${this.formatMoney(e.balance_cents / 100)}</td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>
      </div>
    `;
  },

  renderReportsCashFlow() {
    const cf = this.data.reportsCashFlow || {
      start_date: this.reportsFilter.cashFlowStart,
      end_date: this.reportsFilter.cashFlowEnd,
      beginning_cash_cents: 0,
      inflows_cents: 0,
      outflows_cents: 0,
      net_cash_flow_cents: 0,
      ending_cash_cents: 0,
      activities: []
    };

    return `
      <div class="reports-toolbar">
        <div class="reports-date-group">
          <span>Cash Statement Period:</span>
          <input type="date" class="reports-date-input" value="${this.reportsFilter.cashFlowStart}" onchange="app.setReportsFilter('cashFlowStart', this.value)">
          <span>to</span>
          <input type="date" class="reports-date-input" value="${this.reportsFilter.cashFlowEnd}" onchange="app.setReportsFilter('cashFlowEnd', this.value)">
          <button class="btn-press btn btn-primary text-xs py-1.5 px-3" onclick="app.reloadCashFlowReport()">
            <i data-lucide="play" class="w-3.5 h-3.5"></i> Run Cash Flow
          </button>
        </div>
        <div class="flex items-center gap-2">
          <button class="btn-press btn btn-secondary text-xs" onclick="app.triggerReportExport('cash-flow', 'csv', app.reportsFilter.cashFlowStart, app.reportsFilter.cashFlowEnd)">
            <i data-lucide="file-spreadsheet" class="w-3.5 h-3.5 text-emerald-600"></i> Export CSV
          </button>
          <button class="btn-press btn btn-secondary text-xs" onclick="app.triggerReportExport('cash-flow', 'pdf', app.reportsFilter.cashFlowStart, app.reportsFilter.cashFlowEnd)">
            <i data-lucide="file-text" class="w-3.5 h-3.5 text-rose-600"></i> Vector PDF
          </button>
        </div>
      </div>

      <!-- Cash Metric Cards -->
      <div class="stats-grid mb-6">
        <div class="stat-card mockup-item">
          <div class="stat-title">Beginning Cash</div>
          <div class="stat-value">${this.formatMoney(cf.beginning_cash_cents / 100)}</div>
          <div class="stat-trend text-slate-500">Balance at ${cf.start_date}</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Cash Inflows (+)</div>
          <div class="stat-value" style="color:#059669;">+${this.formatMoney(cf.inflows_cents / 100)}</div>
          <div class="stat-trend trend-up">Collections & Receipts</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Cash Outflows (-)</div>
          <div class="stat-value" style="color:#DC2626;">-${this.formatMoney(cf.outflows_cents / 100)}</div>
          <div class="stat-trend text-slate-500 font-medium">Vendor & Payroll Payments</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Net Cash Flow</div>
          <div class="stat-value" style="color:${cf.net_cash_flow_cents >= 0 ? '#059669' : '#DC2626'};">${this.formatMoney(cf.net_cash_flow_cents / 100)}</div>
          <div class="stat-trend ${cf.net_cash_flow_cents >= 0 ? 'trend-up' : 'text-rose-500'}">${cf.net_cash_flow_cents >= 0 ? 'Net Inflow' : 'Net Burn'}</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Ending Cash</div>
          <div class="stat-value" style="color:#2563EB;">${this.formatMoney(cf.ending_cash_cents / 100)}</div>
          <div class="stat-trend text-slate-500">Balance at ${cf.end_date}</div>
        </div>
      </div>

      <!-- Activities Table -->
      <div class="table-container mockup-item">
        <table class="data-table">
          <thead>
            <tr>
              <th style="width:140px;">Timestamp</th>
              <th style="width:160px;">Reference</th>
              <th>Activity Description</th>
              <th style="width:140px;">Flow Type</th>
              <th style="width:160px; text-align:right;">Cash Amount</th>
            </tr>
          </thead>
          <tbody>
            ${(cf.activities || []).length > 0 ? (cf.activities || []).map(a => `
              <tr>
                <td class="mono-code text-[11px] text-slate-400">${a.timestamp ? a.timestamp.slice(0, 10) : ''}</td>
                <td class="mono-code">${a.reference_number || '-'}</td>
                <td><strong>${a.description}</strong></td>
                <td>
                  <span class="actor-pill ${a.activity_type === 'inflow' ? 'actor-agent' : 'actor-human'}">
                    ${a.activity_type.toUpperCase()}
                  </span>
                </td>
                <td style="text-align:right; font-family:var(--font-mono); font-weight:700; color:${a.activity_type === 'inflow' ? '#059669' : '#DC2626'};">
                  ${a.activity_type === 'inflow' ? '+' : '-'}${this.formatMoney(a.amount_cents / 100)}
                </td>
              </tr>
            `).join('') : `<tr><td colspan="5" class="text-center py-4 text-slate-400">No cash transactions in selected period</td></tr>`}
          </tbody>
        </table>
      </div>
    `;
  },

  renderReportsAging() {
    const isAR = this.reportsFilter.agingType !== 'ap';
    const rep = isAR ? (this.data.reportsAgingAR || {}) : (this.data.reportsAgingAP || {});
    const items = rep.items || [];

    return `
      <!-- Sub-Toggle between A/R and A/P -->
      <div class="reports-toolbar">
        <div class="flex items-center gap-2">
          <button class="btn-press btn ${isAR ? 'btn-primary' : 'btn-secondary'} text-xs" onclick="app.setReportsFilter('agingType', 'ar'); app.reloadAgingReport();">
            <i data-lucide="arrow-down-left" class="w-3.5 h-3.5"></i> Accounts Receivable (A/R)
          </button>
          <button class="btn-press btn ${!isAR ? 'btn-primary' : 'btn-secondary'} text-xs" onclick="app.setReportsFilter('agingType', 'ap'); app.reloadAgingReport();">
            <i data-lucide="arrow-up-right" class="w-3.5 h-3.5"></i> Accounts Payable (A/P)
          </button>
        </div>
        <div class="reports-date-group">
          <span>Aging As Of:</span>
          <input type="date" class="reports-date-input" value="${this.reportsFilter.agingAsOf}" onchange="app.setReportsFilter('agingAsOf', this.value)">
          <button class="btn-press btn btn-secondary text-xs py-1.5 px-3" onclick="app.reloadAgingReport()">
            <i data-lucide="play" class="w-3.5 h-3.5"></i> Refresh Aging
          </button>
          <button class="btn-press btn btn-secondary text-xs" onclick="app.triggerReportExport(app.reportsFilter.agingType === 'ap' ? 'aging-ap' : 'aging-ar', 'csv', null, null, app.reportsFilter.agingAsOf)">
            <i data-lucide="file-spreadsheet" class="w-3.5 h-3.5 text-emerald-600"></i> CSV
          </button>
          <button class="btn-press btn btn-secondary text-xs" onclick="app.triggerReportExport(app.reportsFilter.agingType === 'ap' ? 'aging-ap' : 'aging-ar', 'pdf', null, null, app.reportsFilter.agingAsOf)">
            <i data-lucide="file-text" class="w-3.5 h-3.5 text-rose-600"></i> PDF
          </button>
        </div>
      </div>

      <!-- 5 Aging Bucket Cards -->
      <div class="aging-buckets-grid">
        <div class="aging-bucket-card bucket-current">
          <div class="aging-bucket-title">Current (Not Due)</div>
          <div class="aging-bucket-amount" style="color:#10B981;">${this.formatMoney((rep.current_cents || 0) / 100)}</div>
          <div class="aging-bucket-meta">On schedule</div>
        </div>
        <div class="aging-bucket-card bucket-1-30">
          <div class="aging-bucket-title">1–30 Days Overdue</div>
          <div class="aging-bucket-amount" style="color:#3B82F6;">${this.formatMoney((rep.days_1_30_cents || 0) / 100)}</div>
          <div class="aging-bucket-meta">Early notice</div>
        </div>
        <div class="aging-bucket-card bucket-31-60">
          <div class="aging-bucket-title">31–60 Days Overdue</div>
          <div class="aging-bucket-amount" style="color:#F59E0B;">${this.formatMoney((rep.days_31_60_cents || 0) / 100)}</div>
          <div class="aging-bucket-meta">Follow-up due</div>
        </div>
        <div class="aging-bucket-card bucket-61-90">
          <div class="aging-bucket-title">61–90 Days Overdue</div>
          <div class="aging-bucket-amount" style="color:#F97316;">${this.formatMoney((rep.days_61_90_cents || 0) / 100)}</div>
          <div class="aging-bucket-meta">Escalation tier</div>
        </div>
        <div class="aging-bucket-card bucket-90-plus">
          <div class="aging-bucket-title">90+ Days (Critical)</div>
          <div class="aging-bucket-amount" style="color:#EF4444;">${this.formatMoney((rep.days_90_plus_cents || 0) / 100)}</div>
          <div class="aging-bucket-meta">High delinquency</div>
        </div>
      </div>

      <!-- Aging Total Banner -->
      <div class="flex items-center justify-between p-4 bg-slate-50 border border-slate-200 rounded-xl mb-6">
        <div>
          <span class="text-xs font-bold text-slate-500 uppercase tracking-wider">Total Outstanding ${isAR ? 'Receivables' : 'Payables'}:</span>
          <span class="text-xl font-extrabold text-brand-textMain ml-2">${this.formatMoney((rep.total_cents || 0) / 100)}</span>
        </div>
        <span class="text-xs text-slate-500">${items.length} open ${isAR ? 'customer invoices' : 'supplier bills'}</span>
      </div>

      <!-- Delinquent / Open Items Drill-Down Table -->
      <div class="table-container mockup-item">
        <table class="data-table">
          <thead>
            <tr>
              <th>${isAR ? 'Customer' : 'Supplier'} Name</th>
              <th style="width:140px;">Document #</th>
              <th style="width:120px;">Issue Date</th>
              <th style="width:120px;">Due Date</th>
              <th style="width:110px;">Overdue</th>
              <th style="width:140px; text-align:right;">Total Amount</th>
              <th style="width:140px; text-align:right;">Balance Due</th>
              <th style="width:130px;">Aging Bracket</th>
            </tr>
          </thead>
          <tbody>
            ${items.length > 0 ? items.map(i => {
              const badgeClass = i.bracket === 'current' ? 'status-delivered' : i.bracket === '1_30' ? 'status-draft' : i.bracket === '31_60' ? 'status-pending' : i.bracket === '61_90' ? 'status-rejected' : 'status-rejected';
              return `
                <tr>
                  <td><strong>${i.entity_name}</strong></td>
                  <td class="mono-code">${i.reference_number}</td>
                  <td class="text-slate-500 text-xs">${i.issue_date}</td>
                  <td class="text-slate-500 text-xs font-medium">${i.due_date}</td>
                  <td>
                    ${i.days_overdue > 0 ? `<span class="text-rose-600 font-bold text-xs">${i.days_overdue} days</span>` : `<span class="text-emerald-600 text-xs">Current</span>`}
                  </td>
                  <td style="text-align:right; font-family:var(--font-mono);">${this.formatMoney(i.total_cents / 100)}</td>
                  <td style="text-align:right; font-family:var(--font-mono); font-weight:800; color:#0F172A;">${this.formatMoney(i.due_cents / 100)}</td>
                  <td>
                    <span class="status-badge ${badgeClass}">${i.bracket.toUpperCase()}</span>
                  </td>
                </tr>
              `;
            }).join('') : `<tr><td colspan="8" class="text-center py-4 text-slate-400">All balances are completely settled. No outstanding items found.</td></tr>`}
          </tbody>
        </table>
      </div>
    `;
  },

  renderReportsInventory() {
    const val = this.data.reportsInventoryValuation || {
      total_cost_valuation_cents: 0,
      total_retail_valuation_cents: 0,
      potential_margin_pct: 0,
      total_skus: 0,
      skus: [],
      warehouses: []
    };
    const turn = this.data.reportsInventoryTurnover || {
      period_days: 90,
      cogs_cents: 0,
      avg_inventory_cents: 0,
      turnover_ratio: 0,
      days_sales_inventory: 0,
      dead_stock_items: []
    };

    const mode = this.reportsFilter.inventoryMode || 'sku';

    return `
      <!-- Valuation & Turnover Metric Cards -->
      <div class="stats-grid mb-6">
        <div class="stat-card mockup-item">
          <div class="stat-title">Cost Valuation (Capital Tied)</div>
          <div class="stat-value" style="color:#2563EB;">${this.formatMoney(val.total_cost_valuation_cents / 100)}</div>
          <div class="stat-trend text-slate-500">${val.total_skus} Active SKUs</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Retail Valuation</div>
          <div class="stat-value" style="color:#059669;">${this.formatMoney(val.total_retail_valuation_cents / 100)}</div>
          <div class="stat-trend trend-up">Potential Gross Value</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Unrealized Margin</div>
          <div class="stat-value">${val.potential_margin_pct.toFixed(1)}%</div>
          <div class="stat-trend trend-up">Spread between Cost & Retail</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Turnover Ratio (90d)</div>
          <div class="stat-value">${turn.turnover_ratio.toFixed(2)}x</div>
          <div class="stat-trend text-slate-500 font-medium">COGS / Avg Stock</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Days Sales of Inv (DSI)</div>
          <div class="stat-value">${turn.days_sales_inventory.toFixed(1)} <span class="text-xs font-semibold text-slate-400">Days</span></div>
          <div class="stat-trend text-slate-500 font-medium">Avg liquidation velocity</div>
        </div>
      </div>

      <!-- Toolbar & Sub-views -->
      <div class="reports-toolbar">
        <div class="flex items-center gap-2">
          <button class="btn-press btn ${mode === 'sku' ? 'btn-primary' : 'btn-secondary'} text-xs" onclick="app.setReportsFilter('inventoryMode', 'sku'); app.render();">
            <i data-lucide="package" class="w-3.5 h-3.5"></i> Valuation by SKU
          </button>
          <button class="btn-press btn ${mode === 'warehouse' ? 'btn-primary' : 'btn-secondary'} text-xs" onclick="app.setReportsFilter('inventoryMode', 'warehouse'); app.render();">
            <i data-lucide="warehouse" class="w-3.5 h-3.5"></i> By Warehouse
          </button>
          <button class="btn-press btn ${mode === 'dead_stock' ? 'btn-primary' : 'btn-secondary'} text-xs" onclick="app.setReportsFilter('inventoryMode', 'dead_stock'); app.render();">
            <i data-lucide="alert-octagon" class="w-3.5 h-3.5 text-rose-500"></i> Dead Stock Alerts (${(turn.dead_stock_items || []).length})
          </button>
        </div>
        <div class="flex items-center gap-2">
          <button class="btn-press btn btn-secondary text-xs" onclick="app.triggerReportExport('inventory', 'csv')">
            <i data-lucide="file-spreadsheet" class="w-3.5 h-3.5 text-emerald-600"></i> Export CSV
          </button>
          <button class="btn-press btn btn-secondary text-xs" onclick="app.triggerReportExport('inventory', 'pdf')">
            <i data-lucide="file-text" class="w-3.5 h-3.5 text-rose-600"></i> Vector PDF
          </button>
        </div>
      </div>

      ${mode === 'sku' ? `
        <div class="table-container mockup-item">
          <table class="data-table">
            <thead>
              <tr>
                <th>SKU Code</th>
                <th>Item Name</th>
                <th>Category</th>
                <th style="width:110px; text-align:right;">Stock Qty</th>
                <th style="width:130px; text-align:right;">Unit Cost</th>
                <th style="width:130px; text-align:right;">Retail Price</th>
                <th style="width:150px; text-align:right;">Cost Valuation</th>
                <th style="width:150px; text-align:right;">Retail Valuation</th>
              </tr>
            </thead>
            <tbody>
              ${(val.skus || []).map(s => `
                <tr>
                  <td class="mono-code">${s.sku_code}</td>
                  <td><strong>${s.name}</strong></td>
                  <td><span class="actor-pill actor-human">${s.category || 'Standard'}</span></td>
                  <td style="text-align:right; font-mono font-bold;">${s.stock_quantity}</td>
                  <td style="text-align:right; font-family:var(--font-mono);">${this.formatMoney(s.cost_price_cents / 100)}</td>
                  <td style="text-align:right; font-family:var(--font-mono);">${this.formatMoney(s.retail_price_cents / 100)}</td>
                  <td style="text-align:right; font-family:var(--font-mono); font-weight:700; color:#2563EB;">${this.formatMoney(s.cost_valuation_cents / 100)}</td>
                  <td style="text-align:right; font-family:var(--font-mono); font-weight:700; color:#059669;">${this.formatMoney(s.retail_valuation_cents / 100)}</td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>
      ` : mode === 'warehouse' ? `
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          ${(val.warehouses || []).map(w => `
            <div class="financial-statement-card">
              <div class="flex items-center justify-between mb-3 pb-2 border-b border-slate-200">
                <div>
                  <h4 class="font-extrabold text-sm text-brand-textMain">${w.warehouse_name}</h4>
                  <p class="text-xs text-slate-400">${w.location || 'Primary Site'}</p>
                </div>
                <div class="w-9 h-9 bg-blue-50 rounded-lg flex items-center justify-center">
                  <i data-lucide="building-2" class="w-4 h-4 text-brand-blue"></i>
                </div>
              </div>
              <div class="space-y-2 text-xs">
                <div class="flex justify-between">
                  <span class="text-slate-500">Distinct SKUs:</span>
                  <strong>${w.total_skus}</strong>
                </div>
                <div class="flex justify-between">
                  <span class="text-slate-500">Total Units:</span>
                  <strong>${w.total_quantity} units</strong>
                </div>
                <div class="flex justify-between pt-2 border-t border-slate-100">
                  <span class="text-slate-500 font-bold">Cost Valuation:</span>
                  <span class="font-mono font-extrabold text-blue-600 text-sm">${this.formatMoney(w.cost_valuation_cents / 100)}</span>
                </div>
              </div>
            </div>
          `).join('')}
        </div>
      ` : `
        <!-- Dead Stock Section -->
        <div class="table-container mockup-item">
          <div class="p-4 bg-rose-50 border-b border-rose-200 flex items-center gap-3">
            <i data-lucide="alert-triangle" class="w-5 h-5 text-rose-600 shrink-0"></i>
            <div>
              <strong class="text-xs text-rose-800">Dead Stock Inactivity Warning (>90 Days Inactive)</strong>
              <p class="text-xs text-rose-700">These SKUs have zero sales movement over the last 90 days, locking up capital.</p>
            </div>
          </div>
          <table class="data-table">
            <thead>
              <tr>
                <th>SKU Code</th>
                <th>Item Name</th>
                <th style="width:120px; text-align:right;">Stock Units</th>
                <th style="width:150px; text-align:right;">Capital Locked</th>
                <th style="width:160px;">Last Activity Date</th>
                <th style="width:140px;">Recommended Action</th>
              </tr>
            </thead>
            <tbody>
              ${(turn.dead_stock_items || []).length > 0 ? (turn.dead_stock_items || []).map(ds => `
                <tr>
                  <td class="mono-code">${ds.sku_code}</td>
                  <td><strong>${ds.name}</strong></td>
                  <td style="text-align:right; font-family:var(--font-mono); font-weight:700;">${ds.stock_quantity}</td>
                  <td style="text-align:right; font-family:var(--font-mono); font-weight:800; color:#DC2626;">${this.formatMoney(ds.valuation_cents / 100)}</td>
                  <td class="text-slate-500 text-xs">${ds.last_activity_date ? ds.last_activity_date.slice(0, 10) : 'None Recorded'}</td>
                  <td><span class="status-badge status-draft">Discount / Clearance</span></td>
                </tr>
              `).join('') : `<tr><td colspan="6" class="text-center py-4 text-emerald-600 font-semibold">✓ Zero dead stock detected! All SKUs have healthy turnover.</td></tr>`}
            </tbody>
          </table>
        </div>
      `}
    `;
  },

  renderReportsSales() {
    const s = this.data.reportsSalesVelocity || {
      total_closed_won_cents: 0,
      won_deal_count: 0,
      lost_deal_count: 0,
      win_loss_ratio: 0,
      avg_deal_size_cents: 0,
      avg_sales_cycle_days: 0,
      velocity_daily_cents: 0,
      stage_distribution: []
    };

    return `
      <!-- Sales Velocity Metric Cards -->
      <div class="stats-grid mb-6">
        <div class="stat-card mockup-item">
          <div class="stat-title">Closed-Won Revenue</div>
          <div class="stat-value" style="color:#059669;">${this.formatMoney(s.total_closed_won_cents / 100)}</div>
          <div class="stat-trend trend-up">${s.won_deal_count} Won Deals</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Win / Loss Ratio</div>
          <div class="stat-value">${s.win_loss_ratio.toFixed(2)}x</div>
          <div class="stat-trend ${s.win_loss_ratio >= 1.0 ? 'trend-up' : 'text-rose-500'}">${s.won_deal_count} Won / ${s.lost_deal_count} Lost</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Average Deal Size</div>
          <div class="stat-value">${this.formatMoney(s.avg_deal_size_cents / 100)}</div>
          <div class="stat-trend text-slate-500 font-medium">Per closed contract</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Avg Cycle Duration</div>
          <div class="stat-value">${s.avg_sales_cycle_days.toFixed(1)} <span class="text-xs font-semibold text-slate-400">Days</span></div>
          <div class="stat-trend text-slate-500">Lead to Close Velocity</div>
        </div>
        <div class="stat-card mockup-item">
          <div class="stat-title">Pipeline Velocity</div>
          <div class="stat-value" style="color:#2563EB;">${this.formatMoney(s.velocity_daily_cents / 100)} <span class="text-xs font-semibold text-slate-400">/day</span></div>
          <div class="stat-trend trend-up">Sales throughput rate</div>
        </div>
      </div>

      <!-- Stage Distribution Funnel Table -->
      <div class="table-container mockup-item mb-6">
        <div class="p-4 border-b border-slate-200 flex justify-between items-center">
          <div>
            <h4 class="font-extrabold text-sm text-brand-textMain">Pipeline Distribution & Stage Health</h4>
            <p class="text-xs text-slate-400">Real-time breakdown of deal volume and dwell time across stages.</p>
          </div>
          <button class="btn-press btn btn-secondary text-xs" onclick="app.triggerReportExport('sales-velocity', 'pdf')">
            <i data-lucide="file-text" class="w-3.5 h-3.5 text-rose-600"></i> Export PDF
          </button>
        </div>
        <table class="data-table">
          <thead>
            <tr>
              <th>Pipeline Stage</th>
              <th style="width:120px; text-align:right;">Deal Count</th>
              <th style="width:160px; text-align:right;">Total Pipeline Value</th>
              <th style="width:150px; text-align:right;">Average Stage Age</th>
              <th style="width:160px;">Conversion Status</th>
            </tr>
          </thead>
          <tbody>
            ${(s.stage_distribution || []).map(st => `
              <tr>
                <td><strong class="uppercase text-xs tracking-wider">${st.stage}</strong></td>
                <td style="text-align:right; font-bold;">${st.deal_count}</td>
                <td style="text-align:right; font-family:var(--font-mono); font-weight:700; color:#2563EB;">${this.formatMoney(st.total_value_cents / 100)}</td>
                <td style="text-align:right; font-family:var(--font-mono);">${st.avg_age_days.toFixed(1)} days</td>
                <td>
                  <span class="status-badge ${st.stage === 'won' ? 'status-delivered' : st.stage === 'lost' ? 'status-rejected' : 'status-approved'}">
                    ${st.stage === 'won' ? 'CLOSED WON' : st.stage === 'lost' ? 'CLOSED LOST' : 'ACTIVE PIPELINE'}
                  </span>
                </td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    `;
  },

  renderReportsExport() {
    return `
      <div class="mb-6">
        <h3 class="text-lg font-extrabold text-brand-textMain mb-1">Universal Document & Financial Export Engine</h3>
        <p class="text-xs text-slate-500">Download cryptographically verifiable CSV, JSON, and vector PDF audit documents.</p>
      </div>

      <div class="export-tile-grid">
        <!-- P&L -->
        <div class="export-tile">
          <div>
            <div class="flex items-center gap-3 mb-2">
              <div class="w-10 h-10 rounded-xl bg-blue-50 flex items-center justify-center text-brand-blue">
                <i data-lucide="trending-up" class="w-5 h-5"></i>
              </div>
              <div>
                <h4 class="font-bold text-sm text-brand-textMain">Profit & Loss Statement</h4>
                <p class="text-xs text-slate-400">Revenue, COGS, OpEx & Net Margin</p>
              </div>
            </div>
          </div>
          <div class="flex gap-2 pt-2 border-t border-slate-100">
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('pnl', 'csv')">
              <i data-lucide="file-spreadsheet" class="w-3.5 h-3.5 text-emerald-600"></i> CSV
            </button>
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('pnl', 'pdf')">
              <i data-lucide="file-text" class="w-3.5 h-3.5 text-rose-600"></i> PDF
            </button>
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('pnl', 'json')">
              <i data-lucide="code" class="w-3.5 h-3.5 text-amber-600"></i> JSON
            </button>
          </div>
        </div>

        <!-- Balance Sheet -->
        <div class="export-tile">
          <div>
            <div class="flex items-center gap-3 mb-2">
              <div class="w-10 h-10 rounded-xl bg-purple-50 flex items-center justify-center text-purple-600">
                <i data-lucide="scale" class="w-5 h-5"></i>
              </div>
              <div>
                <h4 class="font-bold text-sm text-brand-textMain">Balance Sheet & Equity</h4>
                <p class="text-xs text-slate-400">Assets, Liabilities & Equity Verification</p>
              </div>
            </div>
          </div>
          <div class="flex gap-2 pt-2 border-t border-slate-100">
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('balance-sheet', 'csv')">
              <i data-lucide="file-spreadsheet" class="w-3.5 h-3.5 text-emerald-600"></i> CSV
            </button>
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('balance-sheet', 'pdf')">
              <i data-lucide="file-text" class="w-3.5 h-3.5 text-rose-600"></i> PDF
            </button>
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('balance-sheet', 'json')">
              <i data-lucide="code" class="w-3.5 h-3.5 text-amber-600"></i> JSON
            </button>
          </div>
        </div>

        <!-- Aged AR -->
        <div class="export-tile">
          <div>
            <div class="flex items-center gap-3 mb-2">
              <div class="w-10 h-10 rounded-xl bg-emerald-50 flex items-center justify-center text-emerald-600">
                <i data-lucide="clock" class="w-5 h-5"></i>
              </div>
              <div>
                <h4 class="font-bold text-sm text-brand-textMain">Aged Accounts Receivable</h4>
                <p class="text-xs text-slate-400">Aging brackets & delinquent invoices</p>
              </div>
            </div>
          </div>
          <div class="flex gap-2 pt-2 border-t border-slate-100">
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('aging-ar', 'csv')">
              <i data-lucide="file-spreadsheet" class="w-3.5 h-3.5 text-emerald-600"></i> CSV
            </button>
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('aging-ar', 'pdf')">
              <i data-lucide="file-text" class="w-3.5 h-3.5 text-rose-600"></i> PDF
            </button>
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('aging-ar', 'json')">
              <i data-lucide="code" class="w-3.5 h-3.5 text-amber-600"></i> JSON
            </button>
          </div>
        </div>

        <!-- Aged AP -->
        <div class="export-tile">
          <div>
            <div class="flex items-center gap-3 mb-2">
              <div class="w-10 h-10 rounded-xl bg-rose-50 flex items-center justify-center text-rose-600">
                <i data-lucide="clock" class="w-5 h-5"></i>
              </div>
              <div>
                <h4 class="font-bold text-sm text-brand-textMain">Aged Accounts Payable</h4>
                <p class="text-xs text-slate-400">Supplier obligations & overdue bills</p>
              </div>
            </div>
          </div>
          <div class="flex gap-2 pt-2 border-t border-slate-100">
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('aging-ap', 'csv')">
              <i data-lucide="file-spreadsheet" class="w-3.5 h-3.5 text-emerald-600"></i> CSV
            </button>
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('aging-ap', 'pdf')">
              <i data-lucide="file-text" class="w-3.5 h-3.5 text-rose-600"></i> PDF
            </button>
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('aging-ap', 'json')">
              <i data-lucide="code" class="w-3.5 h-3.5 text-amber-600"></i> JSON
            </button>
          </div>
        </div>

        <!-- Inventory Valuation -->
        <div class="export-tile">
          <div>
            <div class="flex items-center gap-3 mb-2">
              <div class="w-10 h-10 rounded-xl bg-amber-50 flex items-center justify-center text-amber-600">
                <i data-lucide="boxes" class="w-5 h-5"></i>
              </div>
              <div>
                <h4 class="font-bold text-sm text-brand-textMain">Inventory Stock Valuation</h4>
                <p class="text-xs text-slate-400">SKU cost, retail & warehouse allocation</p>
              </div>
            </div>
          </div>
          <div class="flex gap-2 pt-2 border-t border-slate-100">
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('inventory', 'csv')">
              <i data-lucide="file-spreadsheet" class="w-3.5 h-3.5 text-emerald-600"></i> CSV
            </button>
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('inventory', 'pdf')">
              <i data-lucide="file-text" class="w-3.5 h-3.5 text-rose-600"></i> PDF
            </button>
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('inventory', 'json')">
              <i data-lucide="code" class="w-3.5 h-3.5 text-amber-600"></i> JSON
            </button>
          </div>
        </div>

        <!-- Client Invoices Register -->
        <div class="export-tile">
          <div>
            <div class="flex items-center gap-3 mb-2">
              <div class="w-10 h-10 rounded-xl bg-indigo-50 flex items-center justify-center text-indigo-600">
                <i data-lucide="receipt" class="w-5 h-5"></i>
              </div>
              <div>
                <h4 class="font-bold text-sm text-brand-textMain">Invoices & Billing Register</h4>
                <p class="text-xs text-slate-400">Full accounts billing history</p>
              </div>
            </div>
          </div>
          <div class="flex gap-2 pt-2 border-t border-slate-100">
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('invoices', 'csv')">
              <i data-lucide="file-spreadsheet" class="w-3.5 h-3.5 text-emerald-600"></i> CSV
            </button>
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('invoices', 'pdf')">
              <i data-lucide="file-text" class="w-3.5 h-3.5 text-rose-600"></i> PDF
            </button>
            <button class="btn btn-secondary text-xs flex-1" onclick="app.triggerReportExport('invoices', 'json')">
              <i data-lucide="code" class="w-3.5 h-3.5 text-amber-600"></i> JSON
            </button>
          </div>
        </div>
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
    modal.querySelector('.modal-window').style.maxWidth = '560px';

    const custOptions = (this.data.customers && this.data.customers.length > 0)
      ? this.data.customers.map(c => `<option value="${c.id}">${c.name} (${c.company || 'Client'})</option>`).join('')
      : `
        <option value="cust_01">Acme Corporation</option>
        <option value="cust_02">Starlight Systems</option>
        <option value="cust_03">Apex Logistics</option>
      `;

    const pipes = this.data.pipelines || [];
    const pipeOptions = pipes.map(p => `<option value="${p.id}" ${p.id === this.activePipelineId ? 'selected' : ''}>${p.name}</option>`).join('');

    const activePipe = pipes.find(p => p.id === this.activePipelineId) || pipes[0];
    const stages = (activePipe && activePipe.stages) ? activePipe.stages : [
      { id: 'lead', name: 'New Leads', probability: 20 },
      { id: 'qualified', name: 'Qualified', probability: 40 },
      { id: 'proposal', name: 'Proposal', probability: 60 },
      { id: 'negotiation', name: 'Negotiation', probability: 80 },
      { id: 'won', name: 'Won', probability: 100 }
    ];
    const stageOptions = stages.map(s => `<option value="${s.id}">${s.name} (${s.probability}%)</option>`).join('');

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:10px; color:var(--text-main); display:flex; align-items:center; gap:8px;">
        <i data-lucide="plus-circle" class="w-5 h-5 text-brand-blue"></i> Create Sales Deal
      </h3>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div>
          <label for="deal-title" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Deal Title</label>
          <input type="text" id="deal-title" class="form-input" placeholder="e.g. Enterprise Cloud ERP Deployment" style="margin-top:4px;">
        </div>
        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="deal-pipeline" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Sales Pipeline</label>
            <select id="deal-pipeline" class="form-input" style="margin-top:4px;" onchange="app.onDealPipelineChange(this.value)">
              ${pipeOptions}
            </select>
          </div>
          <div>
            <label for="deal-stage" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Initial Stage</label>
            <select id="deal-stage" class="form-input" style="margin-top:4px;">
              ${stageOptions}
            </select>
          </div>
        </div>
        <div>
          <label for="deal-customer" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Customer Account</label>
          <select id="deal-customer" class="form-input" style="margin-top:4px;">
            ${custOptions}
          </select>
        </div>
        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="deal-value" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Deal Value ($ USD)</label>
            <input type="number" id="deal-value" class="form-input" value="25000" style="margin-top:4px;">
          </div>
          <div>
            <label for="deal-date" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Target Close Date</label>
            <input type="date" id="deal-date" class="form-input" value="2026-11-30" style="margin-top:4px;">
          </div>
        </div>
        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitNewDeal()">Create Deal</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
    if (window.lucide) lucide.createIcons();
  },

  onDealPipelineChange(pipeId) {
    const pipe = (this.data.pipelines || []).find(p => p.id === pipeId);
    const stageSelect = document.getElementById('deal-stage');
    if (!stageSelect || !pipe || !pipe.stages) return;
    stageSelect.innerHTML = pipe.stages.map(s => `<option value="${s.id}">${s.name} (${s.probability}%)</option>`).join('');
  },

  async submitNewDeal() {
    const title = document.getElementById('deal-title').value.trim();
    if (!title) {
      this.showToast('Please enter a deal title');
      return;
    }
    const pipelineId = document.getElementById('deal-pipeline')?.value || this.activePipelineId;
    const stage = document.getElementById('deal-stage')?.value || 'lead';
    const customerId = document.getElementById('deal-customer')?.value || 'cust_01';
    const value = parseFloat(document.getElementById('deal-value').value) || 0;
    const closeDate = document.getElementById('deal-date').value;

    try {
      await fetch('/api/deals', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title, customer_id: customerId, value, stage, pipeline_id: pipelineId, expected_close_date: closeDate })
      });
      this.showToast('Sales deal successfully created');
      this.closeModal();
      await this.reloadEntity('deals');
      await this.reloadEntity('decayAlerts');
      await this.reloadEntity('summary');
      await this.reloadEntity('auditLogs');
    } catch (err) {
      this.showToast('Failed to create deal: ' + err.message);
    }
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

  async openCustomer360Modal(customerId) {
    if (!customerId) return;
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '880px';

    content.innerHTML = `
      <div style="padding:40px; text-align:center;">
        <i data-lucide="loader-2" class="w-8 h-8 animate-spin text-blue-500" style="margin:0 auto 12px;"></i>
        <div style="font-weight:700; color:var(--text-muted);">Loading Omnichannel Customer 360 Profile...</div>
      </div>
    `;
    modal.classList.add('active');
    if (window.lucide) lucide.createIcons();

    try {
      const res = await fetch(`/api/customers/360?id=${encodeURIComponent(customerId)}`);
      if (!res.ok) throw new Error('Customer profile not found');
      const data = await res.json();
      const c = data.customer || {};
      const kpis = data.kpis || { lifetime_value: 0, total_revenue: 0, unpaid_balance: 0, open_pipeline_value: 0 };
      const acts = data.activities || [];
      const invs = data.invoices || [];
      const deals = data.deals || [];

      content.innerHTML = `
        <div style="display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:16px;">
          <div>
            <div class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-blue-100 text-blue-700 text-[10px] font-bold uppercase tracking-wider mb-2">
              <i data-lucide="user-check" class="w-3 h-3"></i> Customer 360 Profile
            </div>
            <h2 style="font-size:22px; font-weight:800; color:var(--text-main); margin:0;">${c.name}</h2>
            <div style="font-size:13px; color:var(--text-muted); margin-top:2px;">
              <strong>${c.company || 'Individual Client'}</strong> • ${c.email || 'No email registered'} • ${c.phone || 'No phone'}
            </div>
          </div>
          <button class="btn btn-secondary btn-sm" onclick="app.closeModal()">
            <i data-lucide="x" class="w-4 h-4"></i> Close
          </button>
        </div>

        <div style="display:grid; grid-template-columns:repeat(4, 1fr); gap:12px; margin-bottom:24px;">
          <div class="kpi-mini-card">
            <div class="kpi-mini-label">Lifetime Value</div>
            <div class="kpi-mini-value" style="color:#2563EB;">${this.formatMoney(kpis.lifetime_value)}</div>
          </div>
          <div class="kpi-mini-card">
            <div class="kpi-mini-label">Paid Revenue</div>
            <div class="kpi-mini-value" style="color:#059669;">${this.formatMoney(kpis.total_revenue)}</div>
          </div>
          <div class="kpi-mini-card">
            <div class="kpi-mini-label">Outstanding AR</div>
            <div class="kpi-mini-value" style="color:${kpis.unpaid_balance > 0 ? '#D97706' : '#10B981'};">${this.formatMoney(kpis.unpaid_balance)}</div>
          </div>
          <div class="kpi-mini-card">
            <div class="kpi-mini-label">Open Pipeline</div>
            <div class="kpi-mini-value" style="color:#7C3AED;">${this.formatMoney(kpis.open_pipeline_value)}</div>
          </div>
        </div>

        <div style="display:grid; grid-template-columns:1.2fr 0.8fr; gap:24px; max-height:480px; overflow-y:auto; padding-right:6px;">
          <!-- Left Column: Activity Feed & Logger -->
          <div>
            <div style="font-size:13px; font-weight:800; color:var(--text-main); text-transform:uppercase; letter-spacing:0.5px; margin-bottom:12px; display:flex; align-items:center; gap:6px;">
              <i data-lucide="activity" class="w-4 h-4 text-blue-600"></i> Omnichannel Activity Timeline (${acts.length})
            </div>

            <!-- Quick Activity Form -->
            <div style="background:var(--bg-tertiary); border:1px solid var(--border-subtle); border-radius:12px; padding:12px; margin-bottom:20px;">
              <div style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase; margin-bottom:8px;">Log Touchpoint / Interaction</div>
              <div style="display:flex; gap:8px; margin-bottom:8px;">
                <select id="c360-act-type" class="form-input" style="width:110px; height:32px; font-size:11px;">
                  <option value="note">Note</option>
                  <option value="call">Phone Call</option>
                  <option value="meeting">Meeting</option>
                  <option value="email">Email</option>
                </select>
                <input type="text" id="c360-act-summary" class="form-input" placeholder="Brief summary (e.g. Q4 pricing review call)" style="flex:1; height:32px; font-size:12px;">
              </div>
              <textarea id="c360-act-content" class="form-input" placeholder="Meeting minutes, notes, action items..." style="height:55px; font-size:11.5px; margin-bottom:8px;"></textarea>
              <div style="text-align:right;">
                <button class="btn btn-primary btn-sm" onclick="app.submitCustomerActivity('${customerId}')">
                  <i data-lucide="send" class="w-3.5 h-3.5"></i> Save Activity
                </button>
              </div>
            </div>

            <!-- Activity Stream -->
            <div class="timeline-stream">
              ${acts.length === 0 ? `
                <div style="font-size:12px; color:var(--text-muted); padding:10px 0;">No activities logged yet.</div>
              ` : acts.map(a => {
                let icon = 'message-square';
                if (a.activity_type === 'call') icon = 'phone';
                else if (a.activity_type === 'meeting') icon = 'calendar';
                else if (a.activity_type === 'stage_change') icon = 'git-commit';
                else if (a.activity_type === 'invoice_paid' || a.activity_type === 'invoice_sent') icon = 'receipt';
                else if (a.activity_type === 'email') icon = 'mail';
                return `
                  <div class="timeline-item">
                    <div class="timeline-dot"><i data-lucide="${icon}" class="w-3 h-3"></i></div>
                    <div class="timeline-content">
                      <div style="display:flex; justify-content:space-between; align-items:baseline; margin-bottom:3px;">
                        <span style="font-weight:700; font-size:12.5px; color:var(--text-main);">${a.summary}</span>
                        <span style="font-size:10.5px; color:#94A3B8;">${(a.created_at || '').slice(0, 16).replace('T', ' ')}</span>
                      </div>
                      ${a.content ? `<div style="font-size:11.5px; color:var(--text-muted); margin-bottom:4px;">${a.content}</div>` : ''}
                      <div style="font-size:10.5px; color:#94A3B8;">By: <strong>${a.author_name || 'System'}</strong></div>
                    </div>
                  </div>
                `;
              }).join('')}
            </div>
          </div>

          <!-- Right Column: Deals & Invoices -->
          <div>
            <div style="font-size:13px; font-weight:800; color:var(--text-main); text-transform:uppercase; letter-spacing:0.5px; margin-bottom:12px; display:flex; align-items:center; gap:6px;">
              <i data-lucide="briefcase" class="w-4 h-4 text-purple-600"></i> Deals (${deals.length})
            </div>
            <div style="display:flex; flex-direction:column; gap:8px; margin-bottom:24px;">
              ${deals.length === 0 ? `
                <div style="font-size:12px; color:var(--text-muted);">No deals for this customer.</div>
              ` : deals.map(d => `
                <div style="background:var(--bg-card); border:1px solid var(--border-subtle); border-radius:10px; padding:10px 12px; display:flex; justify-content:space-between; align-items:center;">
                  <div>
                    <div style="font-weight:700; font-size:12px; color:var(--text-main);">${d.title}</div>
                    <div style="font-size:11px; color:#94A3B8; text-transform:uppercase;">Stage: ${d.stage} • ${d.probability || 60}%</div>
                  </div>
                  <div style="text-align:right;">
                    <div style="font-weight:800; font-size:12.5px; color:#059669;">${this.formatMoney(d.value)}</div>
                  </div>
                </div>
              `).join('')}
            </div>

            <div style="font-size:13px; font-weight:800; color:var(--text-main); text-transform:uppercase; letter-spacing:0.5px; margin-bottom:12px; display:flex; align-items:center; gap:6px;">
              <i data-lucide="receipt" class="w-4 h-4 text-emerald-600"></i> Invoices (${invs.length})
            </div>
            <div style="display:flex; flex-direction:column; gap:8px;">
              ${invs.length === 0 ? `
                <div style="font-size:12px; color:var(--text-muted);">No invoices issued yet.</div>
              ` : invs.map(inv => `
                <div style="background:var(--bg-card); border:1px solid var(--border-subtle); border-radius:10px; padding:10px 12px; display:flex; justify-content:space-between; align-items:center;">
                  <div>
                    <div style="font-weight:700; font-size:12px; color:var(--text-main);">${inv.invoice_number}</div>
                    <div style="font-size:10.5px; color:#94A3B8;">Due: ${inv.due_date}</div>
                    <span class="badge" style="font-size:9.5px; padding:1px 6px; margin-top:2px; background:${inv.status === 'paid' ? '#ECFDF5' : '#FEF3C7'}; color:${inv.status === 'paid' ? '#047857' : '#B45309'}; border:1px solid ${inv.status === 'paid' ? '#A7F3D0' : '#FDE68A'};">
                      ${inv.status.toUpperCase()}
                    </span>
                  </div>
                  <div style="text-align:right;">
                    <div style="font-weight:800; font-size:12.5px; color:var(--text-main);">${this.formatMoney(inv.total_amount)}</div>
                    <button class="btn btn-secondary btn-sm" style="font-size:10px; height:24px; padding:0 8px; margin-top:4px;" onclick="app.sendInvoiceEmailAction('${inv.id}')">
                      <i data-lucide="mail" class="w-3 h-3 text-blue-500"></i> Portal Link
                    </button>
                  </div>
                </div>
              `).join('')}
            </div>
          </div>
        </div>
      `;
      if (window.lucide) lucide.createIcons();
    } catch (err) {
      content.innerHTML = `
        <div style="padding:24px; text-align:center;">
          <div style="color:#DC2626; font-weight:700; margin-bottom:8px;">Failed to load Customer 360 Profile</div>
          <div style="font-size:12px; color:var(--text-muted); margin-bottom:16px;">${err.message}</div>
          <button class="btn btn-secondary" onclick="app.closeModal()">Close</button>
        </div>
      `;
      if (window.lucide) lucide.createIcons();
    }
  },

  async submitCustomerActivity(customerId) {
    const actType = document.getElementById('c360-act-type')?.value || 'note';
    const summary = document.getElementById('c360-act-summary')?.value.trim();
    const content = document.getElementById('c360-act-content')?.value.trim();

    if (!summary) {
      this.showToast('Please enter an activity summary');
      return;
    }

    try {
      await fetch('/api/customers/activities', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          customer_id: customerId,
          activity_type: actType,
          summary,
          content,
          author_name: 'Operations Team'
        })
      });
      this.showToast('Activity logged to Customer 360');
      await this.openCustomer360Modal(customerId);
    } catch (err) {
      this.showToast('Failed to log activity: ' + err.message);
    }
  },

  openNewCustomerModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '500px';

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:10px; color:var(--text-main); display:flex; align-items:center; gap:8px;">
        <i data-lucide="user-plus" class="w-5 h-5 text-brand-blue"></i> Add Customer Account
      </h3>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div>
          <label style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Customer / Contact Name</label>
          <input type="text" id="cust-name" class="form-input" placeholder="e.g. Maya Chen" style="margin-top:4px;">
        </div>
        <div>
          <label style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Company / Organization</label>
          <input type="text" id="cust-company" class="form-input" placeholder="e.g. Meridian Innovations" style="margin-top:4px;">
        </div>
        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Email Address</label>
            <input type="email" id="cust-email" class="form-input" placeholder="e.g. m.chen@meridian.io" style="margin-top:4px;">
          </div>
          <div>
            <label style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Phone Number</label>
            <input type="text" id="cust-phone" class="form-input" placeholder="e.g. +1 555-0199" style="margin-top:4px;">
          </div>
        </div>
        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitNewCustomer()">Save Customer</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
    if (window.lucide) lucide.createIcons();
  },

  async submitNewCustomer() {
    const name = document.getElementById('cust-name')?.value.trim();
    if (!name) {
      this.showToast('Please enter customer name');
      return;
    }
    const company = document.getElementById('cust-company')?.value.trim();
    const email = document.getElementById('cust-email')?.value.trim();
    const phone = document.getElementById('cust-phone')?.value.trim();

    try {
      await fetch('/api/customers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, company, email, phone })
      });
      this.showToast(`Customer account '${name}' created`);
      this.closeModal();
      await this.reloadEntity('customers');
      await this.reloadEntity('auditLogs');
    } catch (err) {
      this.showToast('Failed to create customer: ' + err.message);
    }
  },

  simulateInboundLeadModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '520px';

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:8px; color:var(--text-main); display:flex; align-items:center; gap:8px;">
        <i data-lucide="zap" class="w-5 h-5 text-amber-500"></i> Ingest Inbound Lead Webhook
      </h3>
      <p style="font-size:12px; color:var(--text-muted); margin-bottom:14px;">
        Simulates an incoming lead payload sent to the public ingestion endpoint <code class="mono-code">/api/v1/leads/capture</code>.
      </p>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Contact Name</label>
            <input type="text" id="lead-name" class="form-input" value="Alex Rivera" style="margin-top:4px;">
          </div>
          <div>
            <label style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Company</label>
            <input type="text" id="lead-company" class="form-input" value="Nexus Hypercloud" style="margin-top:4px;">
          </div>
        </div>
        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Email</label>
            <input type="email" id="lead-email" class="form-input" value="alex@nexushyper.io" style="margin-top:4px;">
          </div>
          <div>
            <label style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Lead Source</label>
            <select id="lead-source" class="form-input" style="margin-top:4px;">
              <option value="website_demo">Website Demo Form</option>
              <option value="api_partner">API Partner Referral</option>
              <option value="event_summit">Global Tech Summit</option>
              <option value="inbound_chat">Inbound Chat Widget</option>
            </select>
          </div>
        </div>
        <div>
          <label style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Estimated Contract Value ($ USD)</label>
          <input type="number" id="lead-value" class="form-input" value="35000" style="margin-top:4px;">
        </div>
        <div>
          <label style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Inquiry Notes</label>
          <textarea id="lead-notes" class="form-input" style="height:55px; font-size:12px; margin-top:4px;">Requesting enterprise multi-region deployment and SLA quote.</textarea>
        </div>
        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitInboundLead()">Trigger Webhook Ingest</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
    if (window.lucide) lucide.createIcons();
  },

  async submitInboundLead() {
    const name = document.getElementById('lead-name')?.value.trim();
    const company = document.getElementById('lead-company')?.value.trim();
    const email = document.getElementById('lead-email')?.value.trim();
    const source = document.getElementById('lead-source')?.value || 'website_demo';
    const estimated_value = parseFloat(document.getElementById('lead-value')?.value) || 0;
    const notes = document.getElementById('lead-notes')?.value.trim();

    try {
      const res = await fetch('/api/v1/leads/capture', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, company, email, source, estimated_value, notes })
      });
      if (!res.ok) throw new Error('Lead capture failed');
      this.showToast('Inbound lead captured via public webhook');
      this.closeModal();
      await this.reloadEntity('inboundLeads');
      await this.reloadEntity('auditLogs');
    } catch (err) {
      this.showToast('Error: ' + err.message);
    }
  },

  async convertLeadAction(leadId) {
    if (!confirm('Convert this inbound lead into an active Customer account and qualified Sales Deal?')) return;
    try {
      const res = await fetch('/api/leads/convert', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ lead_id: leadId })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: 'Conversion failed' }));
        throw new Error(err.error || 'Conversion failed');
      }
      this.showToast('Lead successfully converted to Customer and Deal!');
      await this.reloadEntity('inboundLeads');
      await this.reloadEntity('customers');
      await this.reloadEntity('deals');
      await this.reloadEntity('auditLogs');
    } catch (err) {
      this.showToast('Error: ' + err.message);
    }
  },

  async sendInvoiceEmailAction(invoiceId) {
    try {
      this.showToast('Dispatching invoice email with client self-service portal link...');
      const res = await fetch('/api/invoices/send-email', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ invoice_id: invoiceId })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: 'Email dispatch failed' }));
        throw new Error(err.error || 'Email dispatch failed');
      }
      const data = await res.json();
      await this.reloadEntity('emailOutbox');
      await this.reloadEntity('auditLogs');

      if (data.portal_url) {
        window.open(data.portal_url, '_blank');
        this.showToast('Transactional email sent & portal link opened in new tab');
      } else {
        this.showToast('Invoice transactional email successfully dispatched');
      }
    } catch (err) {
      this.showToast('Error: ' + err.message);
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

  // ---------------- HR, Shifts, Timeclock & Payroll Modals ----------------
  openNewEmployeeModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '640px';

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:12px; color:var(--text-main);">Onboard New Team Member</h3>
      <div style="display:flex; flex-direction:column; gap:12px; max-height:75vh; overflow-y:auto; padding-right:6px;">
        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="emp-name" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Full Legal Name</label>
            <input type="text" id="emp-name" class="form-input" placeholder="e.g. Jessica Chen">
          </div>
          <div>
            <label for="emp-email" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Email Address</label>
            <input type="email" id="emp-email" class="form-input" placeholder="jessica@example.com">
          </div>
        </div>

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="emp-role" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Role / Job Title</label>
            <input type="text" id="emp-role" class="form-input" value="Operations Lead">
          </div>
          <div>
            <label for="emp-dept" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Department</label>
            <input type="text" id="emp-dept" class="form-input" value="Logistics">
          </div>
        </div>

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="emp-type" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Employment Type</label>
            <select id="emp-type" class="form-input">
              <option value="full_time">Full Time</option>
              <option value="part_time">Part Time</option>
              <option value="contractor">Contractor</option>
            </select>
          </div>
          <div>
            <label for="emp-sal-type" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Compensation Model</label>
            <select id="emp-sal-type" class="form-input" onchange="document.getElementById('monthly-field-box').style.display = this.value === 'monthly' ? 'block' : 'none'; document.getElementById('hourly-field-box').style.display = this.value === 'hourly' ? 'block' : 'none';">
              <option value="hourly">Hourly Wage</option>
              <option value="monthly">Fixed Monthly Salary</option>
            </select>
          </div>
        </div>

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div id="hourly-field-box">
            <label for="emp-rate" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Hourly Rate ($/hr)</label>
            <input type="number" id="emp-rate" class="form-input" value="35.00" step="0.5">
          </div>
          <div id="monthly-field-box" style="display:none;">
            <label for="emp-monthly" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Monthly Salary ($/mo)</label>
            <input type="number" id="emp-monthly" class="form-input" value="6000.00" step="50">
          </div>
          <div>
            <label for="emp-tax-id" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Tax ID (SSN / EIN / PAN)</label>
            <input type="text" id="emp-tax-id" class="form-input" placeholder="e.g. 98-7654321">
          </div>
        </div>

        <div style="border-top:1px dashed var(--border-subtle); padding-top:10px;">
          <span style="font-size:11px; font-weight:800; color:var(--text-muted); text-transform:uppercase; letter-spacing:0.5px; display:block; mb-2;">Direct Deposit Banking (ACH / Routing)</span>
          <div style="display:grid; grid-template-columns:1fr 1fr 1fr; gap:10px; margin-top:8px;">
            <div>
              <label for="emp-bank-name" style="font-size:10.5px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Bank Name</label>
              <input type="text" id="emp-bank-name" class="form-input" placeholder="Chase / HDFC">
            </div>
            <div>
              <label for="emp-routing" style="font-size:10.5px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Routing / IFSC</label>
              <input type="text" id="emp-routing" class="form-input" placeholder="021000021">
            </div>
            <div>
              <label for="emp-acct" style="font-size:10.5px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Account Number</label>
              <input type="text" id="emp-acct" class="form-input" placeholder="987654321098">
            </div>
          </div>
        </div>

        <div style="border-top:1px dashed var(--border-subtle); padding-top:10px;">
          <span style="font-size:11px; font-weight:800; color:var(--text-muted); text-transform:uppercase; letter-spacing:0.5px; display:block; mb-2;">Emergency Contact</span>
          <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px; margin-top:8px;">
            <div>
              <label for="emp-emer-name" style="font-size:10.5px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Contact Name</label>
              <input type="text" id="emp-emer-name" class="form-input" placeholder="Marcus Chen (Spouse)">
            </div>
            <div>
              <label for="emp-emer-phone" style="font-size:10.5px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Contact Phone</label>
              <input type="text" id="emp-emer-phone" class="form-input" placeholder="+1-555-019-2834">
            </div>
          </div>
        </div>

        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitNewEmployee()">Save Employee Profile</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
  },

  async submitNewEmployee() {
    const name = document.getElementById('emp-name')?.value.trim();
    const email = document.getElementById('emp-email')?.value.trim();
    const role = document.getElementById('emp-role')?.value.trim();
    const dept = document.getElementById('emp-dept')?.value.trim();
    const empType = document.getElementById('emp-type')?.value;
    const salType = document.getElementById('emp-sal-type')?.value;
    const rate = parseFloat(document.getElementById('emp-rate')?.value || 0);
    const monthly = parseFloat(document.getElementById('emp-monthly')?.value || 0);
    const taxId = document.getElementById('emp-tax-id')?.value.trim();
    const bankName = document.getElementById('emp-bank-name')?.value.trim();
    const routing = document.getElementById('emp-routing')?.value.trim();
    const acct = document.getElementById('emp-acct')?.value.trim();
    const emerName = document.getElementById('emp-emer-name')?.value.trim();
    const emerPhone = document.getElementById('emp-emer-phone')?.value.trim();

    if (!name || !role) {
      this.showToast('Please enter both name and role');
      return;
    }

    try {
      const res = await fetch('/api/employees', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: name,
          email: email,
          role: role,
          job_title: role,
          department: dept,
          employment_type: empType,
          salary_type: salType,
          hourly_rate: rate,
          monthly_salary_cents: Math.round(monthly * 100),
          tax_id: taxId,
          bank_name: bankName,
          bank_routing: routing,
          bank_account: acct,
          emergency_contact_name: emerName,
          emergency_contact_phone: emerPhone
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'Failed to add employee'));
        return;
      }
      this.showToast(`Employee ${name} onboarded successfully`);
      this.closeModal();
      await this.reloadEntity('employees');
      await this.reloadEntity('auditLogs');
      this.render();
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  openEditEmployeeModal(empId) {
    const emp = (this.data.employees || []).find(e => e.id === empId);
    if (!emp) {
      this.showToast('Employee not found');
      return;
    }

    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '640px';

    const isMonthly = emp.salary_type === 'monthly';

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:12px; color:var(--text-main);">Update Employee Profile</h3>
      <div style="display:flex; flex-direction:column; gap:12px; max-height:75vh; overflow-y:auto; padding-right:6px;">
        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="edit-emp-name" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Full Legal Name</label>
            <input type="text" id="edit-emp-name" class="form-input" value="${emp.name || ''}">
          </div>
          <div>
            <label for="edit-emp-email" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Email Address</label>
            <input type="email" id="edit-emp-email" class="form-input" value="${emp.email || ''}">
          </div>
        </div>

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="edit-emp-role" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Role / Job Title</label>
            <input type="text" id="edit-emp-role" class="form-input" value="${emp.role || emp.job_title || ''}">
          </div>
          <div>
            <label for="edit-emp-dept" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Department</label>
            <input type="text" id="edit-emp-dept" class="form-input" value="${emp.department || ''}">
          </div>
        </div>

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="edit-emp-type" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Employment Type</label>
            <select id="edit-emp-type" class="form-input">
              <option value="full_time" ${emp.employment_type === 'full_time' ? 'selected' : ''}>Full Time</option>
              <option value="part_time" ${emp.employment_type === 'part_time' ? 'selected' : ''}>Part Time</option>
              <option value="contractor" ${emp.employment_type === 'contractor' ? 'selected' : ''}>Contractor</option>
            </select>
          </div>
          <div>
            <label for="edit-emp-sal-type" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Compensation Model</label>
            <select id="edit-emp-sal-type" class="form-input" onchange="document.getElementById('edit-monthly-field-box').style.display = this.value === 'monthly' ? 'block' : 'none'; document.getElementById('edit-hourly-field-box').style.display = this.value === 'hourly' ? 'block' : 'none';">
              <option value="hourly" ${!isMonthly ? 'selected' : ''}>Hourly Wage</option>
              <option value="monthly" ${isMonthly ? 'selected' : ''}>Fixed Monthly Salary</option>
            </select>
          </div>
        </div>

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div id="edit-hourly-field-box" style="display:${isMonthly ? 'none' : 'block'};">
            <label for="edit-emp-rate" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Hourly Rate ($/hr)</label>
            <input type="number" id="edit-emp-rate" class="form-input" value="${emp.hourly_rate || 35.00}" step="0.5">
          </div>
          <div id="edit-monthly-field-box" style="display:${isMonthly ? 'block' : 'none'};">
            <label for="edit-emp-monthly" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Monthly Salary ($/mo)</label>
            <input type="number" id="edit-emp-monthly" class="form-input" value="${((emp.monthly_salary_cents || 0) / 100).toFixed(2)}" step="50">
          </div>
          <div>
            <label for="edit-emp-tax-id" style="font-size:11px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Tax ID (SSN / EIN / PAN)</label>
            <input type="text" id="edit-emp-tax-id" class="form-input" value="${emp.tax_id || ''}">
          </div>
        </div>

        <div style="border-top:1px dashed var(--border-subtle); padding-top:10px;">
          <span style="font-size:11px; font-weight:800; color:var(--text-muted); text-transform:uppercase; letter-spacing:0.5px; display:block; mb-2;">Direct Deposit Banking (ACH / Routing)</span>
          <div style="display:grid; grid-template-columns:1fr 1fr 1fr; gap:10px; margin-top:8px;">
            <div>
              <label for="edit-emp-bank-name" style="font-size:10.5px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Bank Name</label>
              <input type="text" id="edit-emp-bank-name" class="form-input" value="${emp.bank_name || ''}">
            </div>
            <div>
              <label for="edit-emp-routing" style="font-size:10.5px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Routing / IFSC</label>
              <input type="text" id="edit-emp-routing" class="form-input" value="${emp.bank_routing || ''}">
            </div>
            <div>
              <label for="edit-emp-acct" style="font-size:10.5px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Account Number</label>
              <input type="text" id="edit-emp-acct" class="form-input" value="${emp.bank_account || ''}">
            </div>
          </div>
        </div>

        <div style="border-top:1px dashed var(--border-subtle); padding-top:10px;">
          <span style="font-size:11px; font-weight:800; color:var(--text-muted); text-transform:uppercase; letter-spacing:0.5px; display:block; mb-2;">Emergency Contact</span>
          <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px; margin-top:8px;">
            <div>
              <label for="edit-emp-emer-name" style="font-size:10.5px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Contact Name</label>
              <input type="text" id="edit-emp-emer-name" class="form-input" value="${emp.emergency_contact_name || ''}">
            </div>
            <div>
              <label for="edit-emp-emer-phone" style="font-size:10.5px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Contact Phone</label>
              <input type="text" id="edit-emp-emer-phone" class="form-input" value="${emp.emergency_contact_phone || ''}">
            </div>
          </div>
        </div>

        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitEditEmployee('${empId}')">Save Changes</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
  },

  async submitEditEmployee(empId) {
    const name = document.getElementById('edit-emp-name')?.value.trim();
    const email = document.getElementById('edit-emp-email')?.value.trim();
    const role = document.getElementById('edit-emp-role')?.value.trim();
    const dept = document.getElementById('edit-emp-dept')?.value.trim();
    const empType = document.getElementById('edit-emp-type')?.value;
    const salType = document.getElementById('edit-emp-sal-type')?.value;
    const rate = parseFloat(document.getElementById('edit-emp-rate')?.value || 0);
    const monthly = parseFloat(document.getElementById('edit-emp-monthly')?.value || 0);
    const taxId = document.getElementById('edit-emp-tax-id')?.value.trim();
    const bankName = document.getElementById('edit-emp-bank-name')?.value.trim();
    const routing = document.getElementById('edit-emp-routing')?.value.trim();
    const acct = document.getElementById('edit-emp-acct')?.value.trim();
    const emerName = document.getElementById('edit-emp-emer-name')?.value.trim();
    const emerPhone = document.getElementById('edit-emp-emer-phone')?.value.trim();

    if (!name || !role) {
      this.showToast('Please enter both name and role');
      return;
    }

    try {
      const res = await fetch('/api/employees', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: empId,
          name: name,
          email: email,
          role: role,
          job_title: role,
          department: dept,
          employment_type: empType,
          salary_type: salType,
          hourly_rate: rate,
          monthly_salary_cents: Math.round(monthly * 100),
          tax_id: taxId,
          bank_name: bankName,
          bank_routing: routing,
          bank_account: acct,
          emergency_contact_name: emerName,
          emergency_contact_phone: emerPhone
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'Failed to update employee'));
        return;
      }
      this.showToast(`Employee ${name} updated successfully`);
      this.closeModal();
      await this.reloadEntity('employees');
      await this.reloadEntity('auditLogs');
      this.render();
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  openNewShiftModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '500px';

    const empOptions = (this.data.employees || []).map(e => `<option value="${e.id}">${e.name} (${e.role || 'Staff'})</option>`).join('') || '<option value="emp_01">Sarah Jenkins</option>';

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:10px; color:var(--text-main);">Schedule Shift</h3>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <label for="shift-emp" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Employee</label>
        <select id="shift-emp" class="form-input">
          ${empOptions}
        </select>
        
        <label for="shift-date" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Shift Date</label>
        <input type="date" id="shift-date" class="form-input" value="${new Date().toISOString().slice(0, 10)}">

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
      await this.reloadEntity('shiftConflicts');
      await this.reloadEntity('auditLogs');
      this.render();
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  async recordPunch(eventType) {
    if (!this.selectedTimeclockEmpId && this.data.employees && this.data.employees.length > 0) {
      this.selectedTimeclockEmpId = this.data.employees[0].id;
    }
    if (!this.selectedTimeclockEmpId) {
      this.showToast('Please select an employee first');
      return;
    }
    try {
      const res = await fetch('/api/timeclock/event', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          employee_id: this.selectedTimeclockEmpId,
          event_type: eventType,
          notes: 'Web Timeclock Terminal'
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Punch Error: ' + (err.message || 'Failed to record punch'));
        return;
      }
      this.showToast(`Punched ${eventType.replace(/_/g, ' ').toUpperCase()} successfully`);
      await this.reloadTimeclockStatus();
      await this.reloadEntity('shifts');
      await this.reloadEntity('shiftConflicts');
      await this.reloadEntity('auditLogs');
      this.render();
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  async calculatePayrollPreview() {
    const periodInput = document.getElementById('payroll-period-input');
    const period = periodInput ? periodInput.value : (this.activePayrollPeriod || new Date().toISOString().slice(0, 7));
    this.activePayrollPeriod = period;
    try {
      const res = await fetch('/api/payroll/preview?period=' + encodeURIComponent(period));
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Preview Error: ' + (err.message || 'Failed to preview payroll'));
        return;
      }
      this.data.payrollPreview = await res.json();
      this.showToast(`Payroll calculated for ${period}: Gross ${this.formatMoney(this.data.payrollPreview.total_gross_cents / 100)}, Net ${this.formatMoney(this.data.payrollPreview.total_net_cents / 100)}`);
      this.render();
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  async executePayrollRun() {
    const periodInput = document.getElementById('payroll-period-input');
    const period = periodInput ? periodInput.value : (this.activePayrollPeriod || new Date().toISOString().slice(0, 7));
    this.activePayrollPeriod = period;
    try {
      const res = await fetch('/api/payroll/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          period: period,
          notes: 'Automated monthly payroll run for ' + period
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Payroll Disbursal Error: ' + (err.message || 'Failed to disburse payroll'));
        return;
      }
      this.showToast(`Payroll disbursed for ${period}! Balanced general ledger entry posted.`);
      await this.reloadEntity('payrollRuns');
      await this.reloadEntity('payslips');
      await this.reloadEntity('accounts');
      await this.reloadEntity('journalEntries');
      await this.reloadEntity('summary');
      await this.reloadEntity('auditLogs');
      this.render();
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  downloadPayslipPDF(payslipId) {
    if (!payslipId) {
      this.showToast('Invalid payslip ID');
      return;
    }
    window.open('/api/payroll/payslip/pdf?payslip_id=' + encodeURIComponent(payslipId), '_blank');
  },

  openRequestLeaveModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '520px';

    const empOptions = (this.data.employees || []).map(e => `<option value="${e.id}">${e.name} (${e.role || 'Staff'})</option>`).join('') || '<option value="emp_01">Staff</option>';
    const polOptions = (this.data.leavePolicies || []).map(p => `<option value="${p.id}">${p.name} (${p.days_allowed}d)</option>`).join('') || '<option value="pol_vacation">Annual Vacation</option>';

    const today = new Date().toISOString().slice(0, 10);

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:10px; color:var(--text-main);">Submit Leave Request</h3>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <label for="leave-emp" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Employee</label>
        <select id="leave-emp" class="form-input">
          ${empOptions}
        </select>

        <label for="leave-policy" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Leave Policy</label>
        <select id="leave-policy" class="form-input">
          ${polOptions}
        </select>

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="leave-start" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Start Date</label>
            <input type="date" id="leave-start" class="form-input" value="${today}">
          </div>
          <div>
            <label for="leave-end" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">End Date</label>
            <input type="date" id="leave-end" class="form-input" value="${today}">
          </div>
        </div>

        <label for="leave-reason" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Reason / Notes</label>
        <textarea id="leave-reason" class="form-input" style="height:70px; resize:none;" placeholder="Family vacation, medical appointment, etc."></textarea>

        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitLeaveRequest()">Submit Request</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
  },

  async submitLeaveRequest() {
    const empId = document.getElementById('leave-emp')?.value;
    const policyId = document.getElementById('leave-policy')?.value;
    const startDate = document.getElementById('leave-start')?.value;
    const endDate = document.getElementById('leave-end')?.value;
    const reason = document.getElementById('leave-reason')?.value.trim();

    try {
      const res = await fetch('/api/leaves/requests', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          employee_id: empId,
          policy_id: policyId,
          start_date: startDate,
          end_date: endDate,
          reason: reason
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'Failed to submit leave request'));
        return;
      }
      this.showToast('Leave request submitted successfully');
      this.closeModal();
      await this.reloadEntity('leaveRequests');
      await this.reloadEntity('auditLogs');
      this.render();
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  async reviewLeaveRequest(requestId, status) {
    try {
      const res = await fetch('/api/leaves/review', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          request_id: requestId,
          status: status,
          notes: 'Manager ' + status + ' decision'
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'Failed to review leave request'));
        return;
      }
      this.showToast(`Leave request ${status} successfully`);
      await this.reloadEntity('leaveRequests');
      await this.reloadEntity('auditLogs');
      this.render();
    } catch (e) {
      this.showToast('Error: ' + e.message);
    }
  },

  openUploadDocModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    modal.querySelector('.modal-window').style.maxWidth = '520px';

    const empOptions = (this.data.employees || []).map(e => `<option value="${e.id}">${e.name} (${e.role || 'Staff'})</option>`).join('') || '<option value="emp_01">Staff</option>';

    content.innerHTML = `
      <h3 style="font-size:18px; font-weight:800; margin-bottom:10px; color:var(--text-main);">Upload HR Compliance Document</h3>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <label for="doc-emp" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Associated Employee</label>
        <select id="doc-emp" class="form-input">
          ${empOptions}
        </select>

        <label for="doc-title" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Document Title</label>
        <input type="text" id="doc-title" class="form-input" placeholder="e.g. Standard Employment Agreement">

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
          <div>
            <label for="doc-type" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">Document Type</label>
            <select id="doc-type" class="form-input">
              <option value="contract">Employment Contract</option>
              <option value="nda">Non-Disclosure Agreement (NDA)</option>
              <option value="tax_id">Tax ID / Form W-4 / W-9</option>
              <option value="evaluation">Performance Evaluation</option>
            </select>
          </div>
          <div>
            <label for="doc-filename" style="font-size:12px; font-weight:700; color:var(--text-muted); text-transform:uppercase;">File Name</label>
            <input type="text" id="doc-filename" class="form-input" value="employment_contract_signed.pdf">
          </div>
        </div>

        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button class="btn-press btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn-press btn btn-primary" onclick="app.submitUploadDoc()">Save to Vault</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
  },

  async submitUploadDoc() {
    const empId = document.getElementById('doc-emp')?.value;
    const title = document.getElementById('doc-title')?.value.trim();
    const docType = document.getElementById('doc-type')?.value;
    const fileName = document.getElementById('doc-filename')?.value.trim();

    if (!title) {
      this.showToast('Please enter document title');
      return;
    }

    try {
      const res = await fetch('/api/hr/documents', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          employee_id: empId,
          title: title,
          doc_type: docType,
          file_name: fileName || 'document.pdf',
          file_size_bytes: 142800,
          access_level: 'manage_hr'
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }));
        this.showToast('Error: ' + (err.message || 'Failed to upload document'));
        return;
      }
      this.showToast(`Document "${title}" archived in compliance vault`);
      this.closeModal();
      await this.reloadEntity('hrDocuments');
      await this.reloadEntity('auditLogs');
      this.render();
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
      { title: 'Open Intelligence & Financial Reports', tag: 'Navigation', action: () => { app.setTab('reports'); app.closeCommandPalette(); } },
      { title: 'Run Real-Time Profit & Loss (P&L)', tag: 'Reports', action: () => { app.setTab('reports'); app.setReportsSubTab('pnl'); app.closeCommandPalette(); } },
      { title: 'Run Balance Sheet & Equity Equation', tag: 'Reports', action: () => { app.setTab('reports'); app.setReportsSubTab('balance-sheet'); app.closeCommandPalette(); } },
      { title: 'View Aged Receivables (A/R)', tag: 'Reports', action: () => { app.setTab('reports'); app.setReportsSubTab('aging'); app.setReportsFilter('agingType', 'ar'); app.reloadAgingReport(); app.closeCommandPalette(); } },
      { title: 'View Aged Payables (A/P)', tag: 'Reports', action: () => { app.setTab('reports'); app.setReportsSubTab('aging'); app.setReportsFilter('agingType', 'ap'); app.reloadAgingReport(); app.closeCommandPalette(); } },
      { title: 'Analyze Inventory Valuation & Turnover', tag: 'Reports', action: () => { app.setTab('reports'); app.setReportsSubTab('inventory'); app.closeCommandPalette(); } },
      { title: 'Open Universal Document Export Hub', tag: 'Reports', action: () => { app.setTab('reports'); app.setReportsSubTab('export'); app.closeCommandPalette(); } },
      { title: 'Open Shift Schedule & Team Records', tag: 'Navigation', action: () => { app.setTab('hr'); app.closeCommandPalette(); } },
      { title: 'Open Tasks & Delivery Board', tag: 'Navigation', action: () => { app.setTab('tasks'); app.closeCommandPalette(); } },
      { title: 'Open Local Listing Sync (NAP)', tag: 'Navigation', action: () => { app.setTab('presence'); app.closeCommandPalette(); } },
      { title: 'Open Reviews & Reputation Hub', tag: 'Navigation', action: () => { app.setTab('reviews'); app.closeCommandPalette(); } },
      { title: 'Open Approval Inbox', tag: 'Governance', action: () => { app.setTab('approvals'); app.closeCommandPalette(); } },
      { title: 'Open Dual-Actor Audit Trail', tag: 'Audit', action: () => { app.setTab('audit'); app.closeCommandPalette(); } },
      { title: 'Open Autonomous MCP Agent Platform', tag: 'Agent', action: () => { app.setTab('agents'); app.closeCommandPalette(); } },
      { title: 'Inspect Agent Execution Replay & Debugger', tag: 'Agent', action: () => { app.setTab('agents'); app.setAgentSubTab('executions'); app.closeCommandPalette(); } },
      { title: 'Review Human Escalation Queue', tag: 'Agent', action: () => { app.setTab('agents'); app.setAgentSubTab('escalations'); app.closeCommandPalette(); } },
      { title: 'Manage Outbox Webhook Subscriptions', tag: 'Agent', action: () => { app.setTab('agents'); app.setAgentSubTab('webhooks'); app.closeCommandPalette(); } },
      { title: 'Test Prompt Injection Defense & Context', tag: 'Agent', action: () => { app.setTab('agents'); app.setAgentSubTab('safety'); app.closeCommandPalette(); } },
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


  // ==========================================================================
  // Phase 8: Autonomous MCP Agent Platform & Webhooks
  // ==========================================================================
  activeAgentSubTab: 'quotas',
  agentConfigs: [],
  agentExecutions: [],
  agentEscalations: [],
  webhookSubscriptions: [],
  webhookDeliveries: [],
  agentContextMemory: [],
  sanitizeResult: null,

  setAgentSubTab(subTab) {
    this.activeAgentSubTab = subTab;
    this.ensureAgentPlatformData(subTab);
    this.render();
  },

  async ensureAgentPlatformData(subTab = 'quotas') {
    try {
      if (subTab === 'quotas') {
        const res = await fetch('/api/agent/configs');
        if (res.ok) this.agentConfigs = await res.json();
      } else if (subTab === 'executions') {
        const res = await fetch('/api/agent/executions');
        if (res.ok) this.agentExecutions = await res.json();
      } else if (subTab === 'escalations') {
        const res = await fetch('/api/agent/escalations');
        if (res.ok) this.agentEscalations = await res.json();
      } else if (subTab === 'webhooks') {
        const [resSub, resDel] = await Promise.all([
          fetch('/api/webhooks/subscriptions'),
          fetch('/api/webhooks/deliveries')
        ]);
        if (resSub.ok) this.webhookSubscriptions = await resSub.json();
        if (resDel.ok) this.webhookDeliveries = await resDel.json();
      } else if (subTab === 'safety') {
        const res = await fetch('/api/agent/context');
        if (res.ok) this.agentContextMemory = await res.json();
      }
    } catch (e) {
      console.warn('Failed fetching agent platform data:', e);
    }
  },

  renderAgentPlatform() {
    const sub = this.activeAgentSubTab || 'quotas';
    let body = '';
    if (sub === 'quotas') body = this.renderAgentQuotas();
    else if (sub === 'executions') body = this.renderAgentExecutions();
    else if (sub === 'escalations') body = this.renderAgentEscalations();
    else if (sub === 'webhooks') body = this.renderAgentWebhooks();
    else if (sub === 'safety') body = this.renderAgentSafety();

    return `
      <div class="content-header">
        <div>
          <h1 class="text-2xl font-bold text-slate-900 tracking-tight flex items-center gap-2">
            <i data-lucide="bot" class="w-7 h-7 text-indigo-600"></i>
            Autonomous MCP Agent Platform & Webhooks
          </h1>
          <p class="text-sm text-slate-500 mt-0.5">
            Orchestration engine, per-agent execution quotas, signed outbox webhooks, prompt defense, and human-in-the-loop escalations.
          </p>
        </div>
        <div style="display:flex; gap:10px;">
          <button class="btn-press btn btn-secondary" onclick="app.ensureAgentPlatformData(app.activeAgentSubTab).then(() => app.render())">
            <i data-lucide="refresh-cw" class="w-4 h-4"></i> Refresh
          </button>
          <button class="btn-press btn btn-primary" onclick="app.toggleAgentDrawer()">
            <i data-lucide="terminal" class="w-4 h-4"></i> MCP Inspector
          </button>
        </div>
      </div>

      <div class="reports-toolbar" style="margin-bottom: 24px;">
        <div class="reports-subnav">
          <button class="reports-nav-btn ${sub === 'quotas' ? 'active' : ''}" onclick="app.setAgentSubTab('quotas')">
            <i data-lucide="gauge" class="w-4 h-4"></i> Quotas & Budgets
          </button>
          <button class="reports-nav-btn ${sub === 'executions' ? 'active' : ''}" onclick="app.setAgentSubTab('executions')">
            <i data-lucide="history" class="w-4 h-4"></i> Execution Replay
          </button>
          <button class="reports-nav-btn ${sub === 'escalations' ? 'active' : ''}" onclick="app.setAgentSubTab('escalations')">
            <i data-lucide="alert-triangle" class="w-4 h-4 text-amber-500"></i> Human Escalations (${(this.agentEscalations || []).filter(e => e.status === 'pending').length})
          </button>
          <button class="reports-nav-btn ${sub === 'webhooks' ? 'active' : ''}" onclick="app.setAgentSubTab('webhooks')">
            <i data-lucide="webhook" class="w-4 h-4 text-emerald-500"></i> Signed Webhooks
          </button>
          <button class="reports-nav-btn ${sub === 'safety' ? 'active' : ''}" onclick="app.setAgentSubTab('safety')">
            <i data-lucide="shield-alert" class="w-4 h-4 text-rose-500"></i> Prompt Defense & Context
          </button>
        </div>
      </div>

      ${body}
    `;
  },

  renderAgentQuotas() {
    const cfgs = this.agentConfigs || [];
    return `
      <div class="agent-platform-grid">
        ${cfgs.map(c => {
          const usagePct = c.daily_budget_cents > 0 ? Math.min(100, Math.round((c.current_day_usage_cents / c.daily_budget_cents) * 100)) : 0;
          const isWarn = usagePct >= 80;
          return `
            <div class="agent-quota-card">
              <div style="display:flex; justify-content:space-between; align-items:flex-start;">
                <div>
                  <h3 style="font-weight:700; font-size:16px; color:var(--text-main);">${c.name}</h3>
                  <span style="font-size:11px; color:var(--text-dim); font-family:var(--font-mono);">${c.id}</span>
                </div>
                <span class="actor-pill actor-agent">${c.is_active ? 'ACTIVE' : 'DISABLED'}</span>
              </div>

              <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px; margin-top:8px;">
                <div>
                  <div style="font-size:11px; color:var(--text-muted); text-transform:uppercase;">Tool Rate Quota</div>
                  <div style="font-size:16px; font-weight:800; color:var(--text-main);">${c.tool_quota_per_minute} <span style="font-size:11px; font-weight:500;">calls/min</span></div>
                </div>
                <div>
                  <div style="font-size:11px; color:var(--text-muted); text-transform:uppercase;">Concurrency Cap</div>
                  <div style="font-size:16px; font-weight:800; color:var(--text-main);">${c.active_slots || 0} / ${c.concurrent_limit} <span style="font-size:11px; font-weight:500;">slots</span></div>
                </div>
              </div>

              <div style="margin-top:8px;">
                <div style="display:flex; justify-content:space-between; font-size:12px; font-weight:600;">
                  <span>Daily Budget Allocation</span>
                  <span>$${(c.current_day_usage_cents / 100).toFixed(2)} / $${(c.daily_budget_cents / 100).toFixed(2)} (${usagePct}%)</span>
                </div>
                <div class="quota-gauge-bar">
                  <div class="quota-gauge-fill ${isWarn ? 'warning' : ''}" style="width: ${usagePct}%;"></div>
                </div>
              </div>

              <div style="font-size:11px; color:var(--text-dim); margin-top:4px;">
                Last active: ${c.last_used_at ? new Date(c.last_used_at).toLocaleTimeString() : 'Never'}
              </div>
            </div>
          `;
        }).join('')}
      </div>
    `;
  },

  renderAgentExecutions() {
    const execs = this.agentExecutions || [];
    return `
      <div class="card p-5">
        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
          <h3 class="text-base font-bold text-slate-800 flex items-center gap-2">
            <i data-lucide="history" class="w-5 h-5 text-indigo-500"></i>
            Live Agent Execution Stream & Debugger Replay
          </h3>
          <span style="font-size:12px; color:var(--text-muted);">${execs.length} recorded tool runs</span>
        </div>

        <table class="data-table">
          <thead>
            <tr>
              <th>Timestamp</th>
              <th>Tool Name</th>
              <th>Agent Identity</th>
              <th>Duration</th>
              <th>Status</th>
              <th>Arguments & Return Payload</th>
            </tr>
          </thead>
          <tbody>
            ${execs.length === 0 ? '<tr><td colspan="6" style="text-align:center; padding:24px; color:var(--text-muted);">No agent executions recorded yet.</td></tr>' : execs.map(e => `
              <tr>
                <td style="font-size:11px; font-family:var(--font-mono); color:var(--text-dim); white-space:nowrap;">
                  ${new Date(e.created_at).toLocaleTimeString()}
                </td>
                <td>
                  <strong style="color:var(--text-main); font-family:var(--font-mono); font-size:12px;">${e.tool_name}</strong>
                  <div style="font-size:10px; color:var(--text-dim); text-transform:uppercase;">${e.category}</div>
                </td>
                <td>
                  <span class="actor-pill actor-agent">${e.agent_id}</span>
                </td>
                <td style="font-size:12px; font-weight:700; color:var(--text-main);">
                  ${e.duration_ms} ms
                </td>
                <td>
                  <span class="badge ${e.is_error ? 'badge-danger' : 'badge-success'}">${e.is_error ? 'ERROR' : 'SUCCESS'}</span>
                </td>
                <td>
                  <details style="cursor:pointer;">
                    <summary style="font-size:11px; color:var(--primary); font-weight:600;">Inspect JSON Payloads</summary>
                    <div style="display:grid; grid-template-columns:1fr 1fr; gap:8px; margin-top:8px;">
                      <div>
                        <div style="font-size:10px; color:var(--text-dim); font-weight:700; margin-bottom:2px;">ARGUMENTS</div>
                        <pre class="code-block-viewer">${e.arguments_json || '{}'}</pre>
                      </div>
                      <div>
                        <div style="font-size:10px; color:var(--text-dim); font-weight:700; margin-bottom:2px;">RESULT</div>
                        <pre class="code-block-viewer">${e.result_json || '{}'}</pre>
                      </div>
                    </div>
                  </details>
                </td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    `;
  },

  renderAgentEscalations() {
    const escs = this.agentEscalations || [];
    return `
      <div class="card p-5">
        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
          <div>
            <h3 class="text-base font-bold text-slate-800 flex items-center gap-2">
              <i data-lucide="alert-triangle" class="w-5 h-5 text-amber-500"></i>
              Human Operator Escalation Queue
            </h3>
            <p style="font-size:12px; color:var(--text-muted); margin-top:2px;">
              Autonomous agent handoffs triggered by ambiguous requests, policy thresholds, or negative customer sentiment.
            </p>
          </div>
        </div>

        <div style="display:flex; flex-direction:column; gap:14px;">
          ${escs.length === 0 ? '<div style="text-align:center; padding:32px; color:var(--text-muted);">No escalations pending review. All autonomous agents running smoothly.</div>' : escs.map(e => `
            <div style="background:var(--bg-tertiary); border:1px solid var(--border-subtle); border-radius:14px; padding:18px;">
              <div style="display:flex; justify-content:space-between; align-items:flex-start;">
                <div>
                  <div style="display:flex; align-items:center; gap:8px; margin-bottom:4px;">
                    <span class="escalation-badge-${e.status}">${e.status.toUpperCase()}</span>
                    <strong style="font-size:15px; color:var(--text-main);">${e.title}</strong>
                  </div>
                  <div style="font-size:12px; color:var(--text-dim);">
                    Escalated by <span class="actor-pill actor-agent" style="font-size:9px;">${e.agent_id}</span> on ${new Date(e.created_at).toLocaleString()}
                  </div>
                </div>
                ${e.status === 'pending' ? `
                  <div style="display:flex; gap:8px;">
                    <button class="btn-press btn btn-primary btn-sm" onclick="app.resolveAgentEscalation('${e.id}', 'approved')">
                      <i data-lucide="check" class="w-3.5 h-3.5"></i> Approve
                    </button>
                    <button class="btn-press btn btn-secondary btn-sm" onclick="app.resolveAgentEscalation('${e.id}', 'rejected')">
                      <i data-lucide="x" class="w-3.5 h-3.5"></i> Reject
                    </button>
                  </div>
                ` : `<span style="font-size:12px; color:var(--text-muted);">Resolved</span>`}
              </div>

              <div style="margin-top:12px; font-size:13px; color:var(--text-main);">
                <strong>Reason:</strong> <code style="font-family:var(--font-mono); font-size:12px; background:rgba(0,0,0,0.04); padding:2px 6px; border-radius:4px;">${e.reason}</code>
              </div>

              ${e.recommended_action ? `
                <div style="margin-top:6px; font-size:13px; color:var(--text-main);">
                  <strong>Recommended Action:</strong> ${e.recommended_action}
                </div>
              ` : ''}

              <div style="margin-top:10px;">
                <details style="cursor:pointer;">
                  <summary style="font-size:11px; font-weight:600; color:var(--primary);">View Execution Context</summary>
                  <pre class="code-block-viewer" style="margin-top:6px;">${e.input_context_json || '{}'}</pre>
                </details>
              </div>
            </div>
          `).join('')}
        </div>
      </div>
    `;
  },

  renderAgentWebhooks() {
    const subs = this.webhookSubscriptions || [];
    const delivs = this.webhookDeliveries || [];
    return `
      <div style="display:grid; grid-template-columns: 1fr 1fr; gap:20px;">
        <!-- Registered Webhooks -->
        <div class="card p-5">
          <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
            <h3 class="text-base font-bold text-slate-800 flex items-center gap-2">
              <i data-lucide="webhook" class="w-5 h-5 text-emerald-500"></i>
              Active Webhook Subscriptions
            </h3>
            <button class="btn-press btn btn-primary btn-sm" onclick="app.openNewWebhookModal()">
              <i data-lucide="plus" class="w-3.5 h-3.5"></i> Add Webhook
            </button>
          </div>

          <table class="data-table">
            <thead>
              <tr>
                <th>Subscription</th>
                <th>Target URL</th>
                <th>Event Types</th>
                <th>HMAC Secret</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              ${subs.length === 0 ? '<tr><td colspan="5" style="text-align:center; padding:18px; color:var(--text-muted);">No webhook subscribers registered.</td></tr>' : subs.map(s => `
                <tr>
                  <td><strong>${s.name}</strong></td>
                  <td style="font-size:11px; font-family:var(--font-mono); color:var(--primary); word-break:break-all;">${s.url}</td>
                  <td style="font-size:11px; font-family:var(--font-mono);">${s.event_types}</td>
                  <td><span class="webhook-secret-chip">${(s.secret || '').slice(0, 10)}...</span></td>
                  <td>
                    <button class="btn btn-secondary btn-sm" onclick="app.triggerTestWebhookEvent('${s.id}')">Test</button>
                  </td>
                </tr>
              `).join('')}
            </tbody>
          </table>

          <div style="margin-top:16px; padding:14px; background:var(--bg-tertiary); border-radius:12px; border:1px solid var(--border-subtle); display:flex; justify-content:space-between; align-items:center;">
            <div>
              <strong style="font-size:13px; color:var(--text-main);">Outbox Event Simulator</strong>
              <p style="font-size:11px; color:var(--text-muted);">Trigger signed HMAC-SHA256 test events across subscribers.</p>
            </div>
            <button class="btn-press btn btn-primary btn-sm" onclick="app.triggerTestWebhookEvent()">
              <i data-lucide="send" class="w-3.5 h-3.5"></i> Dispatch po.created
            </button>
          </div>
        </div>

        <!-- Deliveries Log -->
        <div class="card p-5">
          <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
            <h3 class="text-base font-bold text-slate-800 flex items-center gap-2">
              <i data-lucide="send" class="w-5 h-5 text-indigo-500"></i>
              Outbox Delivery Attempts & Retries
            </h3>
            <span style="font-size:12px; color:var(--text-muted);">${delivs.length} logged attempts</span>
          </div>

          <table class="data-table">
            <thead>
              <tr>
                <th>Time</th>
                <th>Event Type</th>
                <th>Status</th>
                <th>HTTP Code</th>
                <th>Retries</th>
              </tr>
            </thead>
            <tbody>
              ${delivs.length === 0 ? '<tr><td colspan="5" style="text-align:center; padding:18px; color:var(--text-muted);">No webhook deliveries logged yet.</td></tr>' : delivs.map(d => `
                <tr>
                  <td style="font-size:11px; font-family:var(--font-mono); color:var(--text-dim);">${new Date(d.created_at).toLocaleTimeString()}</td>
                  <td><code style="font-size:11px; font-family:var(--font-mono);">${d.event_type}</code></td>
                  <td>
                    <span class="badge ${d.status === 'delivered' ? 'badge-success' : 'badge-danger'}">${d.status.toUpperCase()}</span>
                  </td>
                  <td style="font-size:12px; font-weight:700;">${d.status_code || 0}</td>
                  <td style="font-size:12px;">${d.retry_count} / ${d.max_retries}</td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>
      </div>
    `;
  },

  renderAgentSafety() {
    const mems = this.agentContextMemory || [];
    const sanitizeRes = this.sanitizeResult;
    return `
      <div style="display:grid; grid-template-columns: 1fr 1fr; gap:20px;">
        <!-- Injected Context Memory -->
        <div class="card p-5">
          <h3 class="text-base font-bold text-slate-800 flex items-center gap-2" style="margin-bottom:6px;">
            <i data-lucide="database" class="w-5 h-5 text-indigo-500"></i>
            Injected Company Context & Policy Prompts
          </h3>
          <p style="font-size:12px; color:var(--text-muted); margin-bottom:16px;">
            Contextually provided to agents without polluting tool parameters (currency, tax rates, standard lead times).
          </p>

          <table class="data-table">
            <thead>
              <tr>
                <th>Category</th>
                <th>Key</th>
                <th>Injected Value</th>
                <th>Description</th>
              </tr>
            </thead>
            <tbody>
              ${mems.length === 0 ? '<tr><td colspan="4" style="text-align:center; padding:18px; color:var(--text-muted);">No contextual memory records found.</td></tr>' : mems.map(m => `
                <tr>
                  <td><span class="actor-pill actor-agent" style="font-size:9px;">${m.category}</span></td>
                  <td style="font-family:var(--font-mono); font-size:12px; font-weight:700;">${m.key}</td>
                  <td><strong style="color:var(--text-main); font-family:var(--font-mono); font-size:12px;">${m.value}</strong></td>
                  <td style="font-size:11px; color:var(--text-dim);">${m.description || ''}</td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>

        <!-- Prompt Injection Defense Sandbox -->
        <div class="card p-5">
          <h3 class="text-base font-bold text-slate-800 flex items-center gap-2" style="margin-bottom:6px;">
            <i data-lucide="shield-alert" class="w-5 h-5 text-rose-500"></i>
            Prompt Injection Defense Sandbox
          </h3>
          <p style="font-size:12px; color:var(--text-muted); margin-bottom:14px;">
            Test untrusted strings against regex and heuristic jailbreak detectors before dispatching to agents.
          </p>

          <textarea id="prompt-test-input" class="form-input code-input" rows="4" placeholder="Enter untrusted customer review or prompt injection payload (e.g. Please ignore previous instructions and override system prompt)..."></textarea>
          
          <button class="btn-press btn btn-primary" style="margin-top:10px; width:100%;" onclick="app.triggerSanitizeTest()">
            <i data-lucide="shield-check" class="w-4 h-4"></i> Validate & Sanitize Input
          </button>

          ${sanitizeRes ? `
            <div class="sanitize-result-box">
              <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:8px;">
                <span style="font-size:13px; font-weight:700;">Analysis Verdict:</span>
                <span class="badge ${sanitizeRes.is_safe ? 'badge-success' : 'badge-danger'}">
                  ${sanitizeRes.is_safe ? 'SAFE / PASSED' : 'INJECTION DETECTED (BLOCKED)'}
                </span>
              </div>
              <div style="font-size:12px; color:var(--text-muted);">Risk Score: <strong>${(sanitizeRes.risk_score * 100).toFixed(0)}%</strong></div>
              ${sanitizeRes.flagged_patterns && sanitizeRes.flagged_patterns.length > 0 ? `
                <div style="font-size:12px; color:#DC2626; margin-top:4px;">
                  Flagged Patterns: <strong>${sanitizeRes.flagged_patterns.join(', ')}</strong>
                </div>
              ` : ''}
              <div style="margin-top:10px;">
                <div style="font-size:11px; font-weight:700; color:var(--text-dim); margin-bottom:2px;">SANITIZED OUTPUT</div>
                <pre class="code-block-viewer">${sanitizeRes.sanitized_text || ''}</pre>
              </div>
            </div>
          ` : ''}
        </div>
      </div>
    `;
  },

  async triggerSanitizeTest() {
    const input = document.getElementById('prompt-test-input');
    const text = input ? input.value : '';
    if (!text) {
      this.showToast('Please enter text to test.');
      return;
    }
    try {
      const res = await fetch('/api/agent/sanitize', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text })
      });
      if (res.ok) {
        this.sanitizeResult = await res.json();
        this.render();
      }
    } catch (e) {
      this.showToast('Error validating prompt: ' + e.message);
    }
  },

  async triggerTestWebhookEvent(subId) {
    try {
      const res = await fetch('/api/webhooks/test-dispatch', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ event_type: 'po.created' })
      });
      if (res.ok) {
        const data = await res.json();
        this.showToast(`Dispatched event to ${data.deliveries_count || 0} subscriber(s).`);
        await this.ensureAgentPlatformData('webhooks');
        this.render();
      }
    } catch (e) {
      this.showToast('Failed dispatching webhook: ' + e.message);
    }
  },

  async resolveAgentEscalation(id, status) {
    try {
      const res = await fetch('/api/agent/escalations/resolve', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id, status, notes: `Resolved by operator (${status})` })
      });
      if (res.ok) {
        this.showToast(`Escalation ${id} marked as ${status}.`);
        await this.ensureAgentPlatformData('escalations');
        this.render();
      }
    } catch (e) {
      this.showToast('Error resolving escalation: ' + e.message);
    }
  },

  openNewWebhookModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    if (!modal || !content) return;

    content.innerHTML = `
      <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
        <h3 class="text-lg font-bold text-slate-800">Add Webhook Subscription</h3>
        <button class="icon-btn" onclick="app.closeModal()"><i data-lucide="x" class="w-5 h-5"></i></button>
      </div>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div>
          <label style="font-size:12px; font-weight:600;">Subscriber Name</label>
          <input type="text" id="wh-name" class="form-input" placeholder="e.g. Claude Desktop Agent Daemon">
        </div>
        <div>
          <label style="font-size:12px; font-weight:600;">Endpoint URL</label>
          <input type="url" id="wh-url" class="form-input" placeholder="https://agent.example.com/webhook">
        </div>
        <div>
          <label style="font-size:12px; font-weight:600;">HMAC Secret (leave blank to auto-generate)</label>
          <input type="text" id="wh-secret" class="form-input" placeholder="whsec_...">
        </div>
        <div>
          <label style="font-size:12px; font-weight:600;">Subscribed Events (comma separated or * for all)</label>
          <input type="text" id="wh-events" class="form-input" value="po.created, deal.won, invoice.overdue, stock.low">
        </div>
        <button class="btn-press btn btn-primary" style="margin-top:10px;" onclick="app.submitNewWebhook()">Register Webhook</button>
      </div>
    `;
    modal.classList.add('active');
    if (window.lucide) lucide.createIcons();
  },

  async submitNewWebhook() {
    const name = document.getElementById('wh-name')?.value || 'Agent Webhook';
    const url = document.getElementById('wh-url')?.value;
    const secret = document.getElementById('wh-secret')?.value;
    const events = document.getElementById('wh-events')?.value || '*';

    if (!url) {
      this.showToast('Target URL is required.');
      return;
    }

    try {
      const res = await fetch('/api/webhooks/subscriptions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, url, secret, event_types: events })
      });
      if (res.ok) {
        this.closeModal();
        this.showToast('Webhook registered successfully.');
        await this.ensureAgentPlatformData('webhooks');
        this.render();
      }
    } catch (e) {
      this.showToast('Error creating webhook: ' + e.message);
    }
  },


  // ==========================================================================
  // Phase 9: Enterprise Multi-Tenancy, Auth & Compliance
  // ==========================================================================
  async loadOrganizations() {
    try {
      const res = await fetch('/api/orgs');
      if (res.ok) {
        const orgs = await res.json();
        this.data.organizations = orgs || [];
        const sel = document.getElementById('tenant-org-select');
        if (sel && this.data.organizations.length > 0) {
          sel.innerHTML = this.data.organizations.map(o => {
            const orgId = o.org_id || o.id;
            const orgName = o.org_name || o.name || orgId;
            const orgSlug = o.org_slug || o.slug || '';
            return `<option value="${orgId}">${orgName}${orgSlug ? ` (${orgSlug})` : ''}</option>`;
          }).join('');
          const activeOrg = localStorage.getItem('aosphere_active_org') || (this.data.organizations[0].org_id || this.data.organizations[0].id);
          sel.value = activeOrg;
        }
      }
    } catch (err) {
      console.warn('Failed loading organizations:', err);
    }
  },

  async switchOrganization(orgId) {
    if (!orgId) return;
    try {
      const res = await fetch('/api/auth/switch-org', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ org_id: orgId })
      });
      if (res.ok) {
        const data = await res.json();
        localStorage.setItem('aosphere_active_org', orgId);
        const sel = document.getElementById('tenant-org-select');
        if (sel) sel.value = orgId;
        this.showToast(`Switched active tenant to ${data.org_name || orgId}`);
        await this.loadAllData();
        if (this.activeTab === 'compliance') {
          await this.ensureComplianceData(this.activeComplianceSubTab);
        }
        this.render();
      } else {
        const err = await res.json().catch(() => ({}));
        this.showToast(`Failed switching tenant: ${err.error || res.statusText}`);
      }
    } catch (err) {
      this.showToast(`Error switching tenant: ${err.message}`);
    }
  },

  async setComplianceSubTab(subTab) {
    this.activeComplianceSubTab = subTab;
    await this.ensureComplianceData(subTab);
    this.render();
    if (window.lucide) lucide.createIcons();
  },

  async ensureComplianceData(subTab) {
    try {
      if (subTab === 'tenancy') {
        const [orgsRes, invRes] = await Promise.all([
          fetch('/api/orgs'),
          fetch('/api/auth/invitations')
        ]);
        if (orgsRes.ok) this.data.organizations = await orgsRes.json();
        if (invRes.ok) this.data.userInvitations = await invRes.json();
      } else if (subTab === 'auth') {
        const ssoRes = await fetch('/api/auth/sso/configs');
        if (ssoRes.ok) this.data.ssoConfigs = await ssoRes.json();
      } else if (subTab === 'roles') {
        const rolesRes = await fetch('/api/auth/roles');
        if (rolesRes.ok) this.data.customRoles = await rolesRes.json();
      } else if (subTab === 'vault') {
        const [vaultRes, verifyRes] = await Promise.all([
          fetch('/api/compliance/audit-vault?limit=50'),
          fetch('/api/compliance/audit-vault/verify', { method: 'POST' })
        ]);
        if (vaultRes.ok) this.data.auditVaultBlocks = await vaultRes.json();
        if (verifyRes.ok) this.data.auditVaultIntegrity = await verifyRes.json();
      } else if (subTab === 'gdpr') {
        const gdprRes = await fetch('/api/compliance/gdpr/requests');
        if (gdprRes.ok) this.data.privacyRequests = await gdprRes.json();
      } else if (subTab === 'soc2') {
        const soc2Res = await fetch('/api/compliance/soc2/access-review');
        if (soc2Res.ok) this.data.soc2Reviews = await soc2Res.json();
      }
    } catch (err) {
      console.warn('Failed ensuring compliance data:', err);
    }
  },

  renderCompliance() {
    const sub = this.activeComplianceSubTab || 'tenancy';
    let body = '';
    if (sub === 'tenancy') body = this.renderComplianceTenancy();
    else if (sub === 'auth') body = this.renderComplianceAuth();
    else if (sub === 'roles') body = this.renderComplianceRoles();
    else if (sub === 'vault') body = this.renderComplianceVault();
    else if (sub === 'gdpr') body = this.renderComplianceGDPR();
    else if (sub === 'soc2') body = this.renderComplianceSOC2();

    return `
      <div class="content-header">
        <div>
          <h1 class="text-2xl font-bold text-slate-900 tracking-tight flex items-center gap-2">
            <i data-lucide="shield" class="w-7 h-7 text-indigo-600"></i>
            Enterprise Multi-Tenancy, Auth & Compliance
          </h1>
          <p class="text-sm text-slate-500 mt-0.5">
            Strict tenant isolation, RFC 6238 TOTP 2FA, SSO/SAML identity, immutable SHA-256 audit vault, GDPR data rights, and SOC2 Type II readiness.
          </p>
        </div>
        <div style="display:flex; gap:10px;">
          <button class="btn-press btn btn-secondary" onclick="app.verifyAuditVaultChain()">
            <i data-lucide="shield-check" class="w-4 h-4 text-emerald-600"></i> Verify Vault Chain
          </button>
          <button class="btn-press btn btn-secondary" onclick="app.ensureComplianceData(app.activeComplianceSubTab).then(() => app.render())">
            <i data-lucide="refresh-cw" class="w-4 h-4"></i> Refresh
          </button>
        </div>
      </div>

      <div class="reports-toolbar" style="margin-bottom: 24px;">
        <div class="reports-subnav">
          <button class="reports-nav-btn ${sub === 'tenancy' ? 'active' : ''}" onclick="app.setComplianceSubTab('tenancy')">
            <i data-lucide="building-2" class="w-4 h-4"></i> Tenancy & Invitations
          </button>
          <button class="reports-nav-btn ${sub === 'auth' ? 'active' : ''}" onclick="app.setComplianceSubTab('auth')">
            <i data-lucide="key" class="w-4 h-4"></i> 2FA & SSO Identity
          </button>
          <button class="reports-nav-btn ${sub === 'roles' ? 'active' : ''}" onclick="app.setComplianceSubTab('roles')">
            <i data-lucide="user-check" class="w-4 h-4"></i> Granular Roles (${(this.data.customRoles || []).length})
          </button>
          <button class="reports-nav-btn ${sub === 'vault' ? 'active' : ''}" onclick="app.setComplianceSubTab('vault')">
            <i data-lucide="link" class="w-4 h-4 text-emerald-600"></i> Immutable Audit Vault
          </button>
          <button class="reports-nav-btn ${sub === 'gdpr' ? 'active' : ''}" onclick="app.setComplianceSubTab('gdpr')">
            <i data-lucide="file-text" class="w-4 h-4"></i> GDPR & Privacy
          </button>
          <button class="reports-nav-btn ${sub === 'soc2' ? 'active' : ''}" onclick="app.setComplianceSubTab('soc2')">
            <i data-lucide="check-circle" class="w-4 h-4 text-indigo-600"></i> SOC2 Type II
          </button>
        </div>
      </div>

      ${body}
    `;
  },

  renderComplianceTenancy() {
    const orgs = this.data.organizations || [];
    const invs = this.data.userInvitations || [];
    const activeOrgId = localStorage.getItem('aosphere_active_org') || (orgs[0] ? (orgs[0].org_id || orgs[0].id) : 'org_default');

    return `
      <div style="display:flex; flex-direction:column; gap:24px;">
        <!-- Organizations Section -->
        <div class="table-container mockup-item">
          <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
            <div>
              <h3 style="font-size:16px; font-weight:800; color:#0F172A;">Tenant Organizations</h3>
              <p style="font-size:13px; color:#64748B;">Scoped operational domains with zero cross-tenant data leakage.</p>
            </div>
            <button class="btn-press btn btn-primary btn-sm" onclick="app.openNewOrgModal()">
              <i data-lucide="plus" class="w-3.5 h-3.5"></i> Add Organization
            </button>
          </div>
          <table class="data-table">
            <thead>
              <tr>
                <th>Tenant ID</th>
                <th>Organization Name</th>
                <th>Slug</th>
                <th>Role in Org</th>
                <th>Status</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              ${orgs.length === 0 ? '<tr><td colspan="6" style="text-align:center; padding:20px; color:#94A3B8;">No organizations configured</td></tr>' : ''}
              ${orgs.map(o => {
                const oId = o.org_id || o.id;
                const isCurrent = oId === activeOrgId;
                return `
                  <tr>
                    <td><span class="mono-code">${oId}</span></td>
                    <td><strong style="color:#0F172A;">${o.org_name || o.name}</strong></td>
                    <td><span class="badge" style="background:#F1F5F9; color:#475569;">${o.org_slug || o.slug}</span></td>
                    <td><span class="actor-pill actor-human">${o.role || 'admin'}</span></td>
                    <td>
                      ${isCurrent ?
                        '<span class="chain-status-verified"><i data-lucide="check" class="w-3 h-3"></i> Active Scope</span>' :
                        '<span class="badge" style="background:#F1F5F9; color:#64748B;">Available</span>'
                      }
                    </td>
                    <td>
                      ${isCurrent ?
                        '<span style="font-size:12px; color:#64748B; font-weight:600;">Currently Active</span>' :
                        `<button class="btn-press btn btn-secondary btn-sm" onclick="app.switchOrganization('${oId}')">Switch to Tenant</button>`
                      }
                    </td>
                  </tr>
                `;
              }).join('')}
            </tbody>
          </table>
        </div>

        <!-- Invitations Section -->
        <div class="table-container mockup-item">
          <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
            <div>
              <h3 style="font-size:16px; font-weight:800; color:#0F172A;">Pending & Active Invitations</h3>
              <p style="font-size:13px; color:#64748B;">Invite team members with pre-assigned roles and expiring security tokens.</p>
            </div>
            <button class="btn-press btn btn-primary btn-sm" onclick="app.openNewInvitationModal()">
              <i data-lucide="user-plus" class="w-3.5 h-3.5"></i> Invite Member
            </button>
          </div>
          <table class="data-table">
            <thead>
              <tr>
                <th>Invitee Email</th>
                <th>Assigned Role</th>
                <th>Status</th>
                <th>Invited At</th>
                <th>Expires At</th>
                <th>Invite Token</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              ${invs.length === 0 ? '<tr><td colspan="7" style="text-align:center; padding:20px; color:#94A3B8;">No active invitations</td></tr>' : ''}
              ${invs.map(inv => `
                <tr>
                  <td><strong style="color:#0F172A;">${inv.email}</strong></td>
                  <td><span class="actor-pill actor-human">${inv.role}</span></td>
                  <td>
                    <span class="badge" style="background:${inv.status === 'accepted' ? '#ECFDF5; color:#059669;' : '#FFFBEB; color:#D97706;'} font-weight:700;">
                      ${inv.status.toUpperCase()}
                    </span>
                  </td>
                  <td style="font-family:var(--font-mono); font-size:11.5px; color:#64748B;">${new Date(inv.created_at).toLocaleDateString()}</td>
                  <td style="font-family:var(--font-mono); font-size:11.5px; color:#64748B;">${new Date(inv.expires_at).toLocaleDateString()}</td>
                  <td>
                    <button class="btn-press btn btn-secondary btn-sm" style="font-family:var(--font-mono); font-size:11px;" onclick="app.copyInvitationToken('${inv.token}')">
                      <i data-lucide="copy" class="w-3 h-3"></i> Copy Token
                    </button>
                  </td>
                  <td>
                    ${inv.status === 'pending' ?
                      `<button class="btn-press btn btn-secondary btn-sm text-red-600" onclick="app.cancelInvitation('${inv.id}')">Revoke</button>` :
                      `<span style="font-size:12px; color:#94A3B8;">Accepted</span>`
                    }
                  </td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>
      </div>
    `;
  },

  renderComplianceAuth() {
    const ssoConfigs = this.data.ssoConfigs || [];

    return `
      <div style="display:grid; grid-template-columns: repeat(auto-fit, minmax(420px, 1fr)); gap:24px;">
        <!-- Two-Factor Authentication Card -->
        <div class="table-container mockup-item" style="padding:24px;">
          <div style="display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:16px;">
            <div>
              <div class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-emerald-100 text-emerald-800 text-[11px] font-bold uppercase tracking-wider mb-2">
                <i data-lucide="shield-check" class="w-3.5 h-3.5"></i> RFC 6238 Engine
              </div>
              <h3 style="font-size:17px; font-weight:800; color:#0F172A;">Two-Factor Authentication (2FA)</h3>
              <p style="font-size:13px; color:#64748B; margin-top:4px;">
                Pure standard-library Time-based One-Time Passwords (TOTP). Compatible with Google Authenticator, 1Password, and Authy.
              </p>
            </div>
          </div>

          <div style="background:#F8FAFC; border:1px solid var(--border-subtle); border-radius:12px; padding:18px; margin-bottom:18px;">
            <div style="display:flex; align-items:center; justify-content:space-between;">
              <div>
                <div style="font-weight:700; font-size:14px; color:#0F172A;">TOTP Status for Active Admin</div>
                <div style="font-size:12px; color:#64748B; margin-top:2px;">Includes ±30s drift tolerance & 8 emergency recovery codes</div>
              </div>
              <span class="chain-status-verified">
                <i data-lucide="check-circle" class="w-3.5 h-3.5"></i> Ready / Configured
              </span>
            </div>
          </div>

          <div style="display:flex; gap:10px;">
            <button class="btn-press btn btn-primary" onclick="app.setup2FA()">
              <i data-lucide="smartphone" class="w-4 h-4"></i> Setup / Reconfigure 2FA
            </button>
            <button class="btn-press btn btn-secondary text-red-600" onclick="app.disable2FA()">
              Disable 2FA
            </button>
          </div>
        </div>

        <!-- Single Sign-On (SSO / SAML / OIDC) Card -->
        <div class="table-container mockup-item" style="padding:24px;">
          <div style="display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:16px;">
            <div>
              <div class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-indigo-100 text-indigo-800 text-[11px] font-bold uppercase tracking-wider mb-2">
                <i data-lucide="globe" class="w-3.5 h-3.5"></i> Identity Federation
              </div>
              <h3 style="font-size:17px; font-weight:800; color:#0F172A;">Enterprise SSO (OIDC & SAML 2.0)</h3>
              <p style="font-size:13px; color:#64748B; margin-top:4px;">
                Federate identity via Okta, Azure AD (Entra ID), or Google Workspace with automated JIT user provisioning.
              </p>
            </div>
            <button class="btn-press btn btn-primary btn-sm" onclick="app.openNewSSOModal()">
              <i data-lucide="plus" class="w-3.5 h-3.5"></i> Add SSO Provider
            </button>
          </div>

          <div style="display:flex; flex-direction:column; gap:12px;">
            ${ssoConfigs.length === 0 ? '<div style="text-align:center; padding:24px; color:#94A3B8; font-size:13px;">No SSO providers configured yet</div>' : ''}
            ${ssoConfigs.map(sso => `
              <div style="display:flex; justify-content:space-between; align-items:center; padding:14px; background:#F8FAFC; border:1px solid var(--border-subtle); border-radius:10px;">
                <div>
                  <div style="display:flex; align-items:center; gap:8px;">
                    <span class="sso-provider-badge">${sso.provider_type.toUpperCase()}</span>
                    <strong style="color:#0F172A; font-size:14px;">${sso.provider_name}</strong>
                  </div>
                  <div style="font-size:12px; color:#64748B; margin-top:4px;">
                    Domain: <code style="background:#E2E8F0; padding:1px 5px; border-radius:4px;">${sso.domain_mapping || 'All domains'}</code>
                  </div>
                </div>
                <div style="display:flex; gap:6px;">
                  <button class="btn-press btn btn-secondary btn-sm" onclick="app.simulateSSOLogin('${sso.id}')">Simulate Auth</button>
                  <button class="btn-press btn btn-secondary btn-sm text-red-600" onclick="app.deleteSSOConfig('${sso.id}')">Delete</button>
                </div>
              </div>
            `).join('')}
          </div>
        </div>
      </div>
    `;
  },

  renderComplianceRoles() {
    const roles = this.data.customRoles || [];

    return `
      <div class="table-container mockup-item">
        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
          <div>
            <h3 style="font-size:16px; font-weight:800; color:#0F172A;">Custom Granular Roles & Permissions</h3>
            <p style="font-size:13px; color:#64748B;">Define bespoke permission profiles combining ERP, CRM, Finance, HR, and Compliance access flags.</p>
          </div>
          <button class="btn-press btn btn-primary btn-sm" onclick="app.openNewCustomRoleModal()">
            <i data-lucide="plus" class="w-3.5 h-3.5"></i> Create Custom Role
          </button>
        </div>
        <table class="data-table">
          <thead>
            <tr>
              <th>Role Identifier</th>
              <th>Display Name</th>
              <th>Description</th>
              <th>Granted Permissions</th>
              <th>Created</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            ${roles.length === 0 ? '<tr><td colspan="6" style="text-align:center; padding:20px; color:#94A3B8;">No custom roles created. Standard built-in roles active (admin, manager, operator, auditor, human, agent).</td></tr>' : ''}
            ${roles.map(r => {
              const perms = (r.permissions_json || '').replace(/[\[\]"]/g, '').split(',').map(p => p.trim()).filter(Boolean);
              return `
                <tr>
                  <td><span class="mono-code">${r.name}</span></td>
                  <td><strong style="color:#0F172A;">${r.name}</strong></td>
                  <td style="color:#64748B; font-size:13px;">${r.description || '—'}</td>
                  <td>
                    <div style="display:flex; flex-wrap:wrap; gap:4px;">
                      ${perms.map(p => `<span class="badge" style="background:#EEF2FF; color:#4F46E5; font-size:11px;">${p}</span>`).join('')}
                    </div>
                  </td>
                  <td style="font-family:var(--font-mono); font-size:11.5px; color:#64748B;">${new Date(r.created_at).toLocaleDateString()}</td>
                  <td>
                    <button class="btn-press btn btn-secondary btn-sm text-red-600" onclick="app.deleteCustomRole('${r.id}')">Delete</button>
                  </td>
                </tr>
              `;
            }).join('')}
          </tbody>
        </table>
      </div>
    `;
  },

  renderComplianceVault() {
    const blocks = this.data.auditVaultBlocks || [];
    const integrity = this.data.auditVaultIntegrity || { is_valid: true, blocks_verified: blocks.length };

    return `
      <div style="display:flex; flex-direction:column; gap:20px;">
        <!-- Vault Integrity Status Banner -->
        <div style="background:${integrity.is_valid ? 'rgba(16, 185, 129, 0.08)' : 'rgba(239, 68, 68, 0.08)'}; border:1px solid ${integrity.is_valid ? '#10B981' : '#EF4444'}; border-radius:14px; padding:20px; display:flex; justify-content:space-between; align-items:center;">
          <div style="display:flex; align-items:center; gap:16px;">
            <div style="width:44px; height:44px; border-radius:12px; background:${integrity.is_valid ? '#10B981' : '#EF4444'}; color:#FFF; display:flex; align-items:center; justify-content:center;">
              <i data-lucide="${integrity.is_valid ? 'shield-check' : 'alert-octagon'}" class="w-6 h-6"></i>
            </div>
            <div>
              <h3 style="font-size:16px; font-weight:800; color:#0F172A; margin:0;">
                ${integrity.is_valid ? 'CRYPTOGRAPHIC CHAIN INTEGRITY: 100% VERIFIED' : 'INTEGRITY BREACH DETECTED'}
              </h3>
              <p style="font-size:13px; color:#475569; margin:4px 0 0 0;">
                ${integrity.is_valid ?
                  `All ${integrity.blocks_verified} chained SHA-256 blocks traversed in sequence. Genesis root and parental hashes strictly match.` :
                  `Chain hash mismatch error: ${integrity.verification_error || 'Block corruption detected.'}`
                }
              </p>
            </div>
          </div>
          <div style="display:flex; gap:10px;">
            <button class="btn-press btn btn-primary" onclick="app.verifyAuditVaultChain()">
              <i data-lucide="refresh-cw" class="w-4 h-4"></i> Run Traversal Verification
            </button>
            <button class="btn-press btn btn-secondary" onclick="app.openRecordSecurityEventModal()">
              <i data-lucide="plus-circle" class="w-4 h-4"></i> Record Vault Block
            </button>
          </div>
        </div>

        <!-- Chained Vault Ledger Table -->
        <div class="table-container mockup-item">
          <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
            <div>
              <h3 style="font-size:16px; font-weight:800; color:#0F172A;">Chained Security Audit Vault (Immutable Ledger)</h3>
              <p style="font-size:13px; color:#64748B;">Write-only hash-chained provenance trail. Any alteration breaks the hash of every descendant block.</p>
            </div>
            <span class="badge" style="background:#F1F5F9; color:#475569; font-weight:700;">Showing ${blocks.length} Blocks</span>
          </div>
          <table class="data-table">
            <thead>
              <tr>
                <th style="width:60px;">Seq #</th>
                <th>Timestamp</th>
                <th>Event Type</th>
                <th>Actor</th>
                <th>Current Block SHA-256 Hash</th>
                <th>Previous Hash (Parent)</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              ${blocks.length === 0 ? '<tr><td colspan="7" style="text-align:center; padding:20px; color:#94A3B8;">No vault blocks in current organization</td></tr>' : ''}
              ${blocks.map(b => `
                <tr>
                  <td><strong style="color:#0F172A; font-family:var(--font-mono);">#${b.sequence_number}</strong></td>
                  <td style="font-family:var(--font-mono); font-size:11.5px; color:#64748B;">${new Date(b.created_at).toLocaleString()}</td>
                  <td><strong style="color:#0F172A;">${b.event_type}</strong></td>
                  <td>
                    <span class="actor-pill actor-${b.actor_type}">
                      ${b.actor_type.toUpperCase()}: ${b.actor_id}
                    </span>
                  </td>
                  <td>
                    <span class="chain-hash-pill" title="${b.block_hash}" style="cursor:pointer;" onclick="navigator.clipboard.writeText('${b.block_hash}'); app.showToast('Full block hash copied to clipboard');">
                      ${b.block_hash.slice(0, 16)}...
                    </span>
                  </td>
                  <td>
                    <span class="chain-hash-pill" title="${b.prev_hash}">
                      ${b.prev_hash === '0000000000000000000000000000000000000000000000000000000000000000' ? 'GENESIS ROOT' : b.prev_hash.slice(0, 12) + '...'}
                    </span>
                  </td>
                  <td>
                    <span class="chain-status-verified" style="font-size:11px; padding:2px 8px;">
                      <i data-lucide="lock" class="w-3 h-3"></i> Sealed
                    </span>
                  </td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>
      </div>
    `;
  },

  renderComplianceGDPR() {
    const requests = this.data.privacyRequests || [];

    return `
      <div style="display:flex; flex-direction:column; gap:24px;">
        <!-- Data Subject Rights Header -->
        <div class="table-container mockup-item" style="padding:24px;">
          <h3 style="font-size:17px; font-weight:800; color:#0F172A; margin-bottom:6px;">GDPR & Privacy Compliance Suite</h3>
          <p style="font-size:13.5px; color:#64748B; margin-bottom:20px;">
            Full adherence to GDPR Chapter 3. Provide comprehensive data portability packages (Art. 15/20) and automated pseudonymization & erasure (Art. 17 Right to be Forgotten).
          </p>

          <div style="display:grid; grid-template-columns: repeat(auto-fit, minmax(320px, 1fr)); gap:16px;">
            <!-- Export Card -->
            <div style="border:1px solid var(--border-subtle); border-radius:12px; padding:18px; background:#F8FAFC;">
              <div style="display:flex; align-items:center; gap:10px; margin-bottom:10px;">
                <div style="background:#E0E7FF; color:#4338CA; border-radius:8px; padding:6px;">
                  <i data-lucide="download" class="w-5 h-5"></i>
                </div>
                <h4 style="font-weight:700; font-size:15px; color:#0F172A; margin:0;">Article 15: Export My Data</h4>
              </div>
              <p style="font-size:12.5px; color:#64748B; margin-bottom:14px;">
                Generates a standardized JSON archive of all personal identity records, active organization memberships, and activity ledger traces.
              </p>
              <button class="btn-press btn btn-primary" onclick="app.triggerGDPRExport()">
                <i data-lucide="file-json" class="w-4 h-4"></i> Generate JSON Archive
              </button>
            </div>

            <!-- Forget-Me Card -->
            <div style="border:1px solid #FECACA; border-radius:12px; padding:18px; background:#FFF5F5;">
              <div style="display:flex; align-items:center; gap:10px; margin-bottom:10px;">
                <div style="background:#FEE2E2; color:#DC2626; border-radius:8px; padding:6px;">
                  <i data-lucide="user-x" class="w-5 h-5"></i>
                </div>
                <h4 style="font-weight:700; font-size:15px; color:#991B1B; margin:0;">Article 17: Right to be Forgotten</h4>
              </div>
              <p style="font-size:12.5px; color:#7F1D1D; margin-bottom:14px;">
                Irreversibly scrubs personally identifiable data (name, email) while preserving referential ledger safety in accounting records.
              </p>
              <button class="btn-press btn btn-secondary text-red-600" onclick="app.triggerGDPRForgetMe()">
                <i data-lucide="trash-2" class="w-4 h-4"></i> Anonymize Account
              </button>
            </div>
          </div>
        </div>

        <!-- Privacy Request Ledger -->
        <div class="table-container mockup-item">
          <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
            <div>
              <h3 style="font-size:16px; font-weight:800; color:#0F172A;">Privacy Request Log</h3>
              <p style="font-size:13px; color:#64748B;">Audit record of all received GDPR requests and their completion status.</p>
            </div>
          </div>
          <table class="data-table">
            <thead>
              <tr>
                <th>Request ID</th>
                <th>User ID</th>
                <th>Request Type</th>
                <th>Status</th>
                <th>Requested At</th>
                <th>Completed At</th>
              </tr>
            </thead>
            <tbody>
              ${requests.length === 0 ? '<tr><td colspan="6" style="text-align:center; padding:20px; color:#94A3B8;">No privacy requests processed yet</td></tr>' : ''}
              ${requests.map(r => `
                <tr>
                  <td><span class="mono-code">${r.id}</span></td>
                  <td><span class="mono-code">${r.user_id}</span></td>
                  <td><strong style="color:#0F172A;">${r.request_type.toUpperCase()}</strong></td>
                  <td>
                    <span class="badge" style="background:#ECFDF5; color:#047857; font-weight:700;">
                      ${r.status.toUpperCase()}
                    </span>
                  </td>
                  <td style="font-family:var(--font-mono); font-size:11.5px; color:#64748B;">${new Date(r.requested_at).toLocaleString()}</td>
                  <td style="font-family:var(--font-mono); font-size:11.5px; color:#64748B;">${r.completed_at ? new Date(r.completed_at).toLocaleString() : 'Pending'}</td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>
      </div>
    `;
  },

  renderComplianceSOC2() {
    const reviews = this.data.soc2Reviews || [];

    return `
      <div style="display:flex; flex-direction:column; gap:24px;">
        <!-- SOC2 Readiness Controls Checklist -->
        <div style="display:grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap:16px;">
          <div class="soc2-review-card">
            <div style="display:flex; align-items:center; gap:8px; margin-bottom:8px;">
              <i data-lucide="clock" class="w-5 h-5 text-indigo-600"></i>
              <h4 style="font-weight:700; font-size:14px; color:#0F172A; margin:0;">Session Timeout</h4>
            </div>
            <div style="font-size:20px; font-weight:800; color:#0F172A;">30 Minutes</div>
            <div style="font-size:12px; color:#64748B; margin-top:4px;">Enforced idle timeout with automatic token revocation.</div>
          </div>

          <div class="soc2-review-card">
            <div style="display:flex; align-items:center; gap:8px; margin-bottom:8px;">
              <i data-lucide="key" class="w-5 h-5 text-emerald-600"></i>
              <h4 style="font-weight:700; font-size:14px; color:#0F172A; margin:0;">Password Policy</h4>
            </div>
            <div style="font-size:20px; font-weight:800; color:#059669;">SOC2 Compliant</div>
            <div style="font-size:12px; color:#64748B; margin-top:4px;">>= 10 chars, upper, lower, numbers, symbols enforced.</div>
          </div>

          <div class="soc2-review-card">
            <div style="display:flex; align-items:center; gap:8px; margin-bottom:8px;">
              <i data-lucide="layers" class="w-5 h-5 text-blue-600"></i>
              <h4 style="font-weight:700; font-size:14px; color:#0F172A; margin:0;">Tenant Boundary</h4>
            </div>
            <div style="font-size:20px; font-weight:800; color:#2563EB;">Strict Isolation</div>
            <div style="font-size:12px; color:#64748B; margin-top:4px;">All database queries strictly scoped by org_id.</div>
          </div>

          <div class="soc2-review-card">
            <div style="display:flex; align-items:center; gap:8px; margin-bottom:8px;">
              <i data-lucide="shield" class="w-5 h-5 text-purple-600"></i>
              <h4 style="font-weight:700; font-size:14px; color:#0F172A; margin:0;">Ledger Provenance</h4>
            </div>
            <div style="font-size:20px; font-weight:800; color:#7C3AED;">SHA-256 Vault</div>
            <div style="font-size:12px; color:#64748B; margin-top:4px;">Cryptographically chained, immutable security events.</div>
          </div>
        </div>

        <!-- Quarterly Access Review -->
        <div class="table-container mockup-item">
          <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
            <div>
              <h3 style="font-size:16px; font-weight:800; color:#0F172A;">Quarterly User Access Review</h3>
              <p style="font-size:13px; color:#64748B;">SOC2 CC6.1 periodic user privilege audit snapshot and formal executive sign-off.</p>
            </div>
            <button class="btn-press btn btn-primary btn-sm" onclick="app.generateSOC2AccessReview()">
              <i data-lucide="camera" class="w-3.5 h-3.5"></i> Capture Q4 Review Snapshot
            </button>
          </div>

          <div style="display:flex; flex-direction:column; gap:16px;">
            ${reviews.length === 0 ? '<div style="text-align:center; padding:24px; color:#94A3B8;">No access reviews on file. Click above to generate an access snapshot.</div>' : ''}
            ${reviews.map(rev => `
              <div class="soc2-review-card" style="border-left:4px solid ${rev.status === 'signed_off' ? '#10B981' : '#F59E0B'};">
                <div style="display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:12px;">
                  <div>
                    <div style="display:flex; align-items:center; gap:8px;">
                      <h4 style="font-size:16px; font-weight:800; color:#0F172A; margin:0;">Review Quarter: ${rev.quarter}</h4>
                      <span class="badge" style="background:${rev.status === 'signed_off' ? '#ECFDF5; color:#059669;' : '#FFFBEB; color:#D97706;'} font-weight:700;">
                        ${rev.status.toUpperCase()}
                      </span>
                    </div>
                    <div style="font-size:12px; color:#64748B; margin-top:4px;">
                      Snapshot Captured: ${new Date(rev.created_at).toLocaleString()}
                    </div>
                  </div>
                  <div>
                    ${rev.status === 'pending' ?
                      `<button class="btn-press btn btn-primary btn-sm" onclick="app.openSignOffSOC2Modal('${rev.id}')">
                        <i data-lucide="check-square" class="w-3.5 h-3.5"></i> Executive Sign-Off
                      </button>` :
                      `<div style="text-align:right;">
                        <span class="chain-status-verified"><i data-lucide="check" class="w-3 h-3"></i> Signed off by ${rev.reviewed_by}</span>
                        <div style="font-size:11px; color:#64748B; margin-top:3px;">${rev.signed_at ? new Date(rev.signed_at).toLocaleString() : ''}</div>
                      </div>`
                    }
                  </div>
                </div>

                <div style="font-family:var(--font-mono); font-size:11.5px; background:#F8FAFC; border:1px solid var(--border-subtle); padding:10px 14px; border-radius:8px; color:#334155;">
                  <strong>Users Snapshot Payload:</strong>
                  <div style="max-height:120px; overflow-y:auto; margin-top:6px; word-break:break-all;">
                    ${rev.snapshot_json}
                  </div>
                </div>
              </div>
            `).join('')}
          </div>
        </div>
      </div>
    `;
  },

  // ---------------- Modals & Action Handlers ----------------
  openNewOrgModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    if (!modal || !content) return;
    content.innerHTML = `
      <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
        <h3 class="text-lg font-bold text-slate-800">Create New Tenant Organization</h3>
        <button class="icon-btn" onclick="app.closeModal()"><i data-lucide="x" class="w-5 h-5"></i></button>
      </div>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div>
          <label style="font-size:12px; font-weight:600;">Organization Name</label>
          <input type="text" id="new-org-name" class="form-input" placeholder="e.g. AO Sphere APAC Hub">
        </div>
        <div>
          <label style="font-size:12px; font-weight:600;">Slug Identifier</label>
          <input type="text" id="new-org-slug" class="form-input" placeholder="e.g. apac">
        </div>
        <button class="btn-press btn btn-primary" style="margin-top:10px;" onclick="app.submitNewOrg()">Provision Organization</button>
      </div>
    `;
    modal.classList.add('active');
    if (window.lucide) lucide.createIcons();
  },

  async submitNewOrg() {
    const name = document.getElementById('new-org-name')?.value;
    const slug = document.getElementById('new-org-slug')?.value;
    if (!name || !slug) {
      this.showToast('Name and slug are required');
      return;
    }
    try {
      const res = await fetch('/api/orgs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, slug })
      });
      if (res.ok) {
        this.closeModal();
        this.showToast(`Organization '${name}' provisioned successfully`);
        await this.loadOrganizations();
        await this.ensureComplianceData('tenancy');
        this.render();
      }
    } catch (e) {
      this.showToast('Error provisioning organization: ' + e.message);
    }
  },

  openNewInvitationModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    if (!modal || !content) return;
    content.innerHTML = `
      <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
        <h3 class="text-lg font-bold text-slate-800">Invite Team Member</h3>
        <button class="icon-btn" onclick="app.closeModal()"><i data-lucide="x" class="w-5 h-5"></i></button>
      </div>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div>
          <label style="font-size:12px; font-weight:600;">Invitee Email</label>
          <input type="email" id="inv-email" class="form-input" placeholder="colleague@example.com">
        </div>
        <div>
          <label style="font-size:12px; font-weight:600;">Assigned Role</label>
          <select id="inv-role" class="form-input">
            <option value="manager">Manager</option>
            <option value="operator">Operator</option>
            <option value="auditor">Auditor</option>
            <option value="admin">Administrator</option>
          </select>
        </div>
        <button class="btn-press btn btn-primary" style="margin-top:10px;" onclick="app.submitNewInvitation()">Send Invitation</button>
      </div>
    `;
    modal.classList.add('active');
    if (window.lucide) lucide.createIcons();
  },

  async submitNewInvitation() {
    const email = document.getElementById('inv-email')?.value;
    const role = document.getElementById('inv-role')?.value || 'operator';
    if (!email) {
      this.showToast('Email is required');
      return;
    }
    try {
      const res = await fetch('/api/auth/invitations', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, role })
      });
      if (res.ok) {
        this.closeModal();
        this.showToast(`Invitation sent to ${email}`);
        await this.ensureComplianceData('tenancy');
        this.render();
      }
    } catch (e) {
      this.showToast('Error sending invite: ' + e.message);
    }
  },

  async cancelInvitation(id) {
    try {
      const res = await fetch(`/api/auth/invitations?id=${encodeURIComponent(id)}`, { method: 'DELETE' });
      if (res.ok) {
        this.showToast('Invitation revoked');
        await this.ensureComplianceData('tenancy');
        this.render();
      }
    } catch (e) {
      this.showToast('Failed to revoke invitation: ' + e.message);
    }
  },

  copyInvitationToken(token) {
    navigator.clipboard.writeText(token);
    this.showToast('Invitation token copied to clipboard');
  },

  async setup2FA() {
    try {
      const res = await fetch('/api/auth/2fa/setup', { method: 'POST' });
      if (res.ok) {
        const data = await res.json();
        const modal = document.getElementById('action-modal');
        const content = document.getElementById('modal-content');
        if (!modal || !content) return;
        content.innerHTML = `
          <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
            <h3 class="text-lg font-bold text-slate-800">Two-Factor Authentication Setup</h3>
            <button class="icon-btn" onclick="app.closeModal()"><i data-lucide="x" class="w-5 h-5"></i></button>
          </div>
          <div style="display:flex; flex-direction:column; gap:14px;">
            <p style="font-size:13px; color:#475569;">
              Scan this secret into your authenticator application (Google Authenticator, 1Password, Authy), then enter the 6-digit verification code below.
            </p>
            <div style="background:#F8FAFC; border:1px solid var(--border-subtle); padding:14px; border-radius:10px; text-align:center;">
              <div style="font-size:11px; font-weight:700; color:#64748B; text-transform:uppercase; letter-spacing:1px; margin-bottom:6px;">Base32 TOTP Secret Key</div>
              <div style="font-family:var(--font-mono); font-size:16px; font-weight:800; color:#0F172A; letter-spacing:2px;">${data.secret}</div>
            </div>
            <div>
              <label style="font-size:12px; font-weight:600;">Authenticator URI (otpauth://)</label>
              <input type="text" class="form-input" style="font-family:var(--font-mono); font-size:11px;" value="${data.otpauth_uri}" readonly>
            </div>
            <div>
              <label style="font-size:12px; font-weight:600;">Verification Code (6 digits)</label>
              <input type="text" id="totp-verify-code" class="form-input" placeholder="e.g. 123456" maxlength="6" style="font-size:18px; text-align:center; letter-spacing:4px; font-weight:700;">
            </div>
            <button class="btn-press btn btn-primary" onclick="app.verify2FASetup()">Verify & Activate 2FA</button>
          </div>
        `;
        modal.classList.add('active');
        if (window.lucide) lucide.createIcons();
      }
    } catch (e) {
      this.showToast('Failed initiating 2FA setup: ' + e.message);
    }
  },

  async verify2FASetup() {
    const code = document.getElementById('totp-verify-code')?.value;
    if (!code || code.length !== 6) {
      this.showToast('Please enter a 6-digit code');
      return;
    }
    try {
      const res = await fetch('/api/auth/2fa/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code })
      });
      if (res.ok) {
        const data = await res.json();
        const modal = document.getElementById('action-modal');
        const content = document.getElementById('modal-content');
        if (!modal || !content) return;
        const codes = data.recovery_codes || [];
        content.innerHTML = `
          <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
            <h3 class="text-lg font-bold text-slate-800">2FA Activated Successfully</h3>
            <button class="icon-btn" onclick="app.closeModal()"><i data-lucide="x" class="w-5 h-5"></i></button>
          </div>
          <div style="display:flex; flex-direction:column; gap:14px;">
            <div style="background:#ECFDF5; border:1px solid #A7F3D0; border-radius:10px; padding:14px; color:#065F46; font-size:13px;">
              Two-Factor Authentication is now enforced for your account. Save these 8 one-time emergency recovery codes in a secure password manager.
            </div>
            <div class="recovery-codes-grid">
              ${codes.map(c => `<div class="recovery-code-chip">${c}</div>`).join('')}
            </div>
            <div style="display:flex; gap:10px; margin-top:8px;">
              <button class="btn-press btn btn-primary" onclick="navigator.clipboard.writeText('${codes.join(', ')}'); app.showToast('Recovery codes copied');">
                <i data-lucide="copy" class="w-4 h-4"></i> Copy Codes
              </button>
              <button class="btn-press btn btn-secondary" onclick="app.closeModal()">I Have Saved My Codes</button>
            </div>
          </div>
        `;
        if (window.lucide) lucide.createIcons();
      } else {
        const err = await res.json().catch(() => ({}));
        this.showToast('Verification failed: ' + (err.error || 'Invalid TOTP code'));
      }
    } catch (e) {
      this.showToast('Error verifying code: ' + e.message);
    }
  },

  async disable2FA() {
    if (!confirm('Are you sure you want to disable Two-Factor Authentication?')) return;
    try {
      const res = await fetch('/api/auth/2fa/disable', { method: 'POST' });
      if (res.ok) {
        this.showToast('Two-Factor Authentication has been disabled');
        await this.ensureComplianceData('auth');
        this.render();
      }
    } catch (e) {
      this.showToast('Failed to disable 2FA: ' + e.message);
    }
  },

  openNewSSOModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    if (!modal || !content) return;
    content.innerHTML = `
      <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
        <h3 class="text-lg font-bold text-slate-800">Add Enterprise SSO Provider</h3>
        <button class="icon-btn" onclick="app.closeModal()"><i data-lucide="x" class="w-5 h-5"></i></button>
      </div>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div>
          <label style="font-size:12px; font-weight:600;">Provider Display Name</label>
          <input type="text" id="sso-name" class="form-input" placeholder="e.g. Okta Corporate SSO">
        </div>
        <div>
          <label style="font-size:12px; font-weight:600;">Protocol Type</label>
          <select id="sso-type" class="form-input">
            <option value="oidc">OpenID Connect (OIDC)</option>
            <option value="saml">SAML 2.0</option>
          </select>
        </div>
        <div>
          <label style="font-size:12px; font-weight:600;">Domain Mapping</label>
          <input type="text" id="sso-domain" class="form-input" placeholder="e.g. enterprise.com">
        </div>
        <div>
          <label style="font-size:12px; font-weight:600;">Issuer URL</label>
          <input type="url" id="sso-issuer" class="form-input" placeholder="https://login.okta.com/oauth2/default">
        </div>
        <div>
          <label style="font-size:12px; font-weight:600;">SSO Endpoint URL</label>
          <input type="url" id="sso-url" class="form-input" placeholder="https://login.okta.com/oauth2/v1/authorize">
        </div>
        <button class="btn-press btn btn-primary" style="margin-top:10px;" onclick="app.submitNewSSO()">Save SSO Configuration</button>
      </div>
    `;
    modal.classList.add('active');
    if (window.lucide) lucide.createIcons();
  },

  async submitNewSSO() {
    const name = document.getElementById('sso-name')?.value;
    const type = document.getElementById('sso-type')?.value;
    const domain = document.getElementById('sso-domain')?.value;
    const issuer = document.getElementById('sso-issuer')?.value;
    const sso_url = document.getElementById('sso-url')?.value;
    if (!name || !issuer) {
      this.showToast('Provider name and issuer URL are required');
      return;
    }
    try {
      const res = await fetch('/api/auth/sso/configs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          provider_name: name,
          provider_type: type,
          domain_mapping: domain,
          issuer_url: issuer,
          sso_url: sso_url,
          client_id: 'client_' + Math.random().toString(36).substring(7)
        })
      });
      if (res.ok) {
        this.closeModal();
        this.showToast('SSO provider registered');
        await this.ensureComplianceData('auth');
        this.render();
      }
    } catch (e) {
      this.showToast('Error registering SSO: ' + e.message);
    }
  },

  async deleteSSOConfig(id) {
    try {
      const res = await fetch(`/api/auth/sso/configs?id=${encodeURIComponent(id)}`, { method: 'DELETE' });
      if (res.ok) {
        this.showToast('SSO configuration deleted');
        await this.ensureComplianceData('auth');
        this.render();
      }
    } catch (e) {
      this.showToast('Failed deleting SSO config: ' + e.message);
    }
  },

  async simulateSSOLogin(configId) {
    try {
      const res = await fetch('/api/auth/sso/callback', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          config_id: configId,
          email: 'sso_user@enterprise.com',
          name: 'Federated Enterprise User'
        })
      });
      if (res.ok) {
        const data = await res.json();
        this.showToast(`SSO Auth Simulated: User ${data.user_id} authenticated`);
      }
    } catch (e) {
      this.showToast('SSO simulation failed: ' + e.message);
    }
  },

  openNewCustomRoleModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    if (!modal || !content) return;
    content.innerHTML = `
      <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
        <h3 class="text-lg font-bold text-slate-800">Create Granular Custom Role</h3>
        <button class="icon-btn" onclick="app.closeModal()"><i data-lucide="x" class="w-5 h-5"></i></button>
      </div>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div>
          <label style="font-size:12px; font-weight:600;">Role Identifier</label>
          <input type="text" id="role-name" class="form-input" placeholder="e.g. inventory_auditor">
        </div>
        <div>
          <label style="font-size:12px; font-weight:600;">Role Description</label>
          <input type="text" id="role-desc" class="form-input" placeholder="Can inspect stock, warehouse balances, and ledger reports">
        </div>
        <div>
          <label style="font-size:12px; font-weight:600; margin-bottom:6px; display:block;">Select Permissions</label>
          <div style="display:grid; grid-template-columns: 1fr 1fr; gap:8px;">
            <label style="font-size:12px; display:flex; align-items:center; gap:6px;">
              <input type="checkbox" name="role-perm" value="erp:read" checked> ERP: Read Inventory
            </label>
            <label style="font-size:12px; display:flex; align-items:center; gap:6px;">
              <input type="checkbox" name="role-perm" value="erp:write"> ERP: Modify Inventory
            </label>
            <label style="font-size:12px; display:flex; align-items:center; gap:6px;">
              <input type="checkbox" name="role-perm" value="finance:read" checked> Finance: Read Ledger
            </label>
            <label style="font-size:12px; display:flex; align-items:center; gap:6px;">
              <input type="checkbox" name="role-perm" value="finance:write"> Finance: Post Entries
            </label>
            <label style="font-size:12px; display:flex; align-items:center; gap:6px;">
              <input type="checkbox" name="role-perm" value="crm:read"> CRM: Read Deals
            </label>
            <label style="font-size:12px; display:flex; align-items:center; gap:6px;">
              <input type="checkbox" name="role-perm" value="hr:read"> HR: Read Roster
            </label>
            <label style="font-size:12px; display:flex; align-items:center; gap:6px;">
              <input type="checkbox" name="role-perm" value="audit:read" checked> Vault: Audit Read
            </label>
            <label style="font-size:12px; display:flex; align-items:center; gap:6px;">
              <input type="checkbox" name="role-perm" value="compliance:admin"> Compliance: Admin
            </label>
          </div>
        </div>
        <button class="btn-press btn btn-primary" style="margin-top:10px;" onclick="app.submitNewCustomRole()">Save Role Profile</button>
      </div>
    `;
    modal.classList.add('active');
    if (window.lucide) lucide.createIcons();
  },

  async submitNewCustomRole() {
    const name = document.getElementById('role-name')?.value;
    const description = document.getElementById('role-desc')?.value;
    const checked = Array.from(document.querySelectorAll('input[name="role-perm"]:checked')).map(el => el.value);
    if (!name || checked.length === 0) {
      this.showToast('Role name and at least one permission are required');
      return;
    }
    try {
      const res = await fetch('/api/auth/roles', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, description, permissions: checked })
      });
      if (res.ok) {
        this.closeModal();
        this.showToast(`Custom role '${name}' created`);
        await this.ensureComplianceData('roles');
        this.render();
      }
    } catch (e) {
      this.showToast('Error creating custom role: ' + e.message);
    }
  },

  async deleteCustomRole(id) {
    try {
      const res = await fetch(`/api/auth/roles?id=${encodeURIComponent(id)}`, { method: 'DELETE' });
      if (res.ok) {
        this.showToast('Role deleted');
        await this.ensureComplianceData('roles');
        this.render();
      }
    } catch (e) {
      this.showToast('Failed to delete role: ' + e.message);
    }
  },

  async verifyAuditVaultChain() {
    try {
      const res = await fetch('/api/compliance/audit-vault/verify', { method: 'POST' });
      if (res.ok) {
        const data = await res.json();
        this.data.auditVaultIntegrity = data;
        if (data.is_valid) {
          this.showToast(`Vault chain verified: ${data.blocks_verified} blocks intact (SHA-256)`);
        } else {
          this.showToast(`Vault error: ${data.verification_error}`);
        }
        await this.ensureComplianceData('vault');
        this.render();
      }
    } catch (e) {
      this.showToast('Error verifying vault chain: ' + e.message);
    }
  },

  openRecordSecurityEventModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    if (!modal || !content) return;
    content.innerHTML = `
      <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
        <h3 class="text-lg font-bold text-slate-800">Record Security Vault Event</h3>
        <button class="icon-btn" onclick="app.closeModal()"><i data-lucide="x" class="w-5 h-5"></i></button>
      </div>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div>
          <label style="font-size:12px; font-weight:600;">Event Type</label>
          <input type="text" id="vault-event-type" class="form-input" value="sec.policy_change">
        </div>
        <div>
          <label style="font-size:12px; font-weight:600;">Event Details / Payload</label>
          <textarea id="vault-event-details" class="form-input" rows="3" placeholder="Security event description..."></textarea>
        </div>
        <button class="btn-press btn btn-primary" onclick="app.submitRecordSecurityEvent()">Seal into SHA-256 Chain</button>
      </div>
    `;
    modal.classList.add('active');
    if (window.lucide) lucide.createIcons();
  },

  async submitRecordSecurityEvent() {
    const event_type = document.getElementById('vault-event-type')?.value;
    const details = document.getElementById('vault-event-details')?.value || '';
    if (!event_type) return;
    try {
      const res = await fetch('/api/compliance/audit-vault', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ event_type, details })
      });
      if (res.ok) {
        this.closeModal();
        this.showToast('Event cryptographically sealed in Audit Vault');
        await this.ensureComplianceData('vault');
        this.render();
      }
    } catch (e) {
      this.showToast('Failed to record vault block: ' + e.message);
    }
  },

  async triggerGDPRExport() {
    try {
      const res = await fetch('/api/compliance/gdpr/export', { method: 'POST' });
      if (res.ok) {
        const data = await res.json();
        const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `gdpr_personal_data_export_${new Date().toISOString().slice(0, 10)}.json`;
        a.click();
        URL.revokeObjectURL(url);
        this.showToast('GDPR Data Export package downloaded');
        await this.ensureComplianceData('gdpr');
        this.render();
      }
    } catch (e) {
      this.showToast('Failed generating export: ' + e.message);
    }
  },

  async triggerGDPRForgetMe() {
    if (!confirm('WARNING: Article 17 "Right to be Forgotten" will permanently anonymize your user identity records. Proceed?')) return;
    try {
      const res = await fetch('/api/compliance/gdpr/forget-me', { method: 'POST' });
      if (res.ok) {
        const data = await res.json();
        this.showToast('Account personal data successfully anonymized');
        await this.ensureComplianceData('gdpr');
        this.render();
      }
    } catch (e) {
      this.showToast('Error processing forget-me request: ' + e.message);
    }
  },

  async generateSOC2AccessReview() {
    try {
      const res = await fetch('/api/compliance/soc2/access-review/generate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ quarter: '2026-Q4' })
      });
      if (res.ok) {
        this.showToast('Quarterly access review snapshot generated');
        await this.ensureComplianceData('soc2');
        this.render();
      }
    } catch (e) {
      this.showToast('Error generating SOC2 review: ' + e.message);
    }
  },

  openSignOffSOC2Modal(reviewId) {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    if (!modal || !content) return;
    content.innerHTML = `
      <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
        <h3 class="text-lg font-bold text-slate-800">SOC2 Access Review Formal Sign-Off</h3>
        <button class="icon-btn" onclick="app.closeModal()"><i data-lucide="x" class="w-5 h-5"></i></button>
      </div>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div>
          <label style="font-size:12px; font-weight:600;">Sign-Off Officer / Reviewer Name</label>
          <input type="text" id="soc2-officer" class="form-input" value="Chief Information Security Officer">
        </div>
        <div>
          <label style="font-size:12px; font-weight:600;">Attestation Notes</label>
          <textarea id="soc2-notes" class="form-input" rows="3">All active user accounts and privilege grants verified against HR records for Q4. No unauthorized access identified.</textarea>
        </div>
        <button class="btn-press btn btn-primary" onclick="app.submitSignOffSOC2('${reviewId}')">Affirm & Sign Off</button>
      </div>
    `;
    modal.classList.add('active');
    if (window.lucide) lucide.createIcons();
  },

  async submitSignOffSOC2(reviewId) {
    const reviewer = document.getElementById('soc2-officer')?.value || 'Compliance Officer';
    const notes = document.getElementById('soc2-notes')?.value || 'Access verified';
    try {
      const res = await fetch('/api/compliance/soc2/access-review/signoff', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: reviewId, reviewed_by: reviewer, notes })
      });
      if (res.ok) {
        this.closeModal();
        this.showToast('SOC2 Access Review formally signed off');
        await this.ensureComplianceData('soc2');
        this.render();
      }
    } catch (e) {
      this.showToast('Error signing off SOC2 review: ' + e.message);
    }
  },

  // ---------------- Module 13: Backups & Disaster Recovery ----------------
  async ensureBackupData() {
    if (!this.data.backups) {
      try {
        const res = await fetch('/api/backup/list');
        if (res.ok) this.data.backups = await res.json();
      } catch (e) {
        this.data.backups = [];
      }
    }
    if (!this.data.backupConfig) {
      try {
        const res = await fetch('/api/backup/config');
        if (res.ok) this.data.backupConfig = await res.json();
      } catch (e) {
        this.data.backupConfig = { endpoint: '', bucket: '', region: 'us-east-1', access_key: '', secret_key: '' };
      }
    }
  },

  renderBackups() {
    this.ensureBackupData().then(() => {
      const el = document.getElementById('backup-table-body');
      if (el) el.innerHTML = this.renderBackupRows();
      const stats = document.getElementById('backup-stats-container');
      if (stats) stats.innerHTML = this.renderBackupStats();
    });

    return `
      <div class="content-header">
        <div>
          <h1 class="page-title">Disaster Recovery & Encrypted Cloud Backups</h1>
          <p class="page-subtitle">Zero-downtime point-in-time snapshots, AES-256-GCM encryption, and S3 / Cloudflare R2 replication</p>
        </div>
        <div class="header-actions">
          <button class="btn-press btn btn-secondary" onclick="app.data.backups = null; app.ensureBackupData().then(() => app.render())">
            <i data-lucide="refresh-cw" class="w-4 h-4"></i> Refresh
          </button>
          <button class="btn-press btn btn-secondary" onclick="app.toggleBackupConfig()">
            <i data-lucide="cloud" class="w-4 h-4"></i> Cloud S3 / R2 Config
          </button>
          <button class="btn-press btn btn-primary" onclick="app.openCreateBackupModal()">
            <i data-lucide="shield" class="w-4 h-4"></i> Take Instant Snapshot
          </button>
        </div>
      </div>

      <div id="backup-stats-container" class="dashboard-grid mb-6">
        ${this.renderBackupStats()}
      </div>

      <!-- Cloud S3/R2 Config Card (Collapsible) -->
      <div id="backup-config-panel" class="card mb-6" style="display: none;">
        <div class="card-header">
          <div class="card-title"><i data-lucide="cloud" class="w-5 h-5 text-indigo-500"></i> Cloud Storage Replication (AWS S3 / Cloudflare R2 / MinIO)</div>
        </div>
        <div class="card-body">
          <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 16px;">
            <div>
              <label class="form-label">S3 Endpoint URL (leave empty for AWS S3)</label>
              <input type="text" id="s3-endpoint" class="form-input" placeholder="https://<account>.r2.cloudflarestorage.com" value="${(this.data.backupConfig?.endpoint || '')}">
            </div>
            <div>
              <label class="form-label">Bucket Name</label>
              <input type="text" id="s3-bucket" class="form-input" placeholder="aosphere-backups" value="${(this.data.backupConfig?.bucket || '')}">
            </div>
            <div>
              <label class="form-label">Region</label>
              <input type="text" id="s3-region" class="form-input" placeholder="auto or us-east-1" value="${(this.data.backupConfig?.region || 'us-east-1')}">
            </div>
            <div>
              <label class="form-label">Access Key ID</label>
              <input type="text" id="s3-access-key" class="form-input" placeholder="AKIA..." value="${(this.data.backupConfig?.access_key || '')}">
            </div>
            <div>
              <label class="form-label">Secret Access Key</label>
              <input type="password" id="s3-secret-key" class="form-input" placeholder="********" value="${(this.data.backupConfig?.secret_key || '')}">
            </div>
          </div>
          <div style="margin-top: 16px; display: flex; justify-content: flex-end;">
            <button class="btn-press btn btn-primary" onclick="app.saveBackupConfig()">
              <i data-lucide="save" class="w-4 h-4"></i> Save S3 Credentials
            </button>
          </div>
        </div>
      </div>

      <!-- Backups Snapshots Table -->
      <div class="card">
        <div class="card-header">
          <div class="card-title"><i data-lucide="database" class="w-5 h-5 text-emerald-500"></i> Verified Snapshot Archives</div>
          <div class="card-subtitle">WAL checkpoints consolidated with PRAGMA integrity_check verification</div>
        </div>
        <div class="card-body p-0">
          <div class="table-responsive">
            <table class="data-table">
              <thead>
                <tr>
                  <th>Snapshot File</th>
                  <th>Created At</th>
                  <th>Archive Size</th>
                  <th>AES-256-GCM</th>
                  <th>SHA-256 Verification</th>
                  <th>Integrity</th>
                  <th style="text-align: right;">Actions</th>
                </tr>
              </thead>
              <tbody id="backup-table-body">
                ${this.renderBackupRows()}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    `;
  },

  renderBackupStats() {
    const list = this.data.backups || [];
    const totalSize = list.reduce((acc, b) => acc + (b.size_bytes || 0), 0);
    const encCount = list.filter(b => b.is_encrypted).length;
    const latest = list.length > 0 ? new Date(list[0].created_at).toLocaleString() : 'No snapshots yet';

    const fmtBytes = (bytes) => {
      if (bytes === 0) return '0 B';
      const k = 1024;
      const sizes = ['B', 'KB', 'MB', 'GB'];
      const i = Math.floor(Math.log(bytes) / Math.log(k));
      return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    };

    return `
      <div class="stat-card">
        <div class="stat-icon bg-emerald-50 text-emerald-600"><i data-lucide="database" class="w-5 h-5"></i></div>
        <div class="stat-content">
          <div class="stat-label">Verified Snapshots</div>
          <div class="stat-value">${list.length}</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon bg-indigo-50 text-indigo-600"><i data-lucide="lock" class="w-5 h-5"></i></div>
        <div class="stat-content">
          <div class="stat-label">AES-256 Encrypted</div>
          <div class="stat-value">${encCount} / ${list.length}</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon bg-blue-50 text-blue-600"><i data-lucide="hard-drive" class="w-5 h-5"></i></div>
        <div class="stat-content">
          <div class="stat-label">Total Backup Storage</div>
          <div class="stat-value">${fmtBytes(totalSize)}</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon bg-amber-50 text-amber-600"><i data-lucide="clock" class="w-5 h-5"></i></div>
        <div class="stat-content">
          <div class="stat-label">Latest Snapshot</div>
          <div class="stat-value text-xs font-mono" style="word-break: break-all;">${latest}</div>
        </div>
      </div>
    `;
  },

  renderBackupRows() {
    const list = this.data.backups || [];
    if (list.length === 0) {
      return `<tr><td colspan="7" class="text-center p-8 text-slate-500">No backup snapshots found. Click <strong>Take Instant Snapshot</strong> to create your first point-in-time archive.</td></tr>`;
    }

    const fmtBytes = (bytes) => {
      if (!bytes) return '0 B';
      const k = 1024;
      const sizes = ['B', 'KB', 'MB', 'GB'];
      const i = Math.floor(Math.log(bytes) / Math.log(k));
      return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    };

    return list.map(b => {
      const dt = new Date(b.created_at).toLocaleString();
      const shortHash = b.checksum_sha256 ? b.checksum_sha256.substring(0, 12) + '...' : 'Verified';
      const encBadge = b.is_encrypted 
        ? `<span class="badge badge-success"><i data-lucide="lock" class="w-3 h-3 inline"></i> AES-GCM</span>`
        : `<span class="badge badge-secondary">Plain SQLite</span>`;

      return `
        <tr>
          <td><strong class="font-mono text-xs">${b.filename}</strong></td>
          <td><span class="text-xs text-slate-500">${dt}</span></td>
          <td><span class="font-mono text-xs">${fmtBytes(b.size_bytes)}</span></td>
          <td>${encBadge}</td>
          <td><span class="font-mono text-xs text-slate-500" title="${b.checksum_sha256}">${shortHash}</span></td>
          <td><span class="badge badge-success"><i data-lucide="check" class="w-3 h-3 inline"></i> PRAGMA OK</span></td>
          <td style="text-align: right;">
            <button class="btn btn-secondary btn-sm" onclick="app.syncBackupCloud('${b.filename}')" title="Upload to Cloud S3">
              <i data-lucide="cloud-upload" class="w-3 h-3"></i> Sync S3
            </button>
            <button class="btn btn-danger btn-sm" onclick="app.openRestoreBackupModal('${b.filename}', ${b.is_encrypted})" style="margin-left: 6px;">
              <i data-lucide="rotate-ccw" class="w-3 h-3"></i> Restore
            </button>
          </td>
        </tr>
      `;
    }).join('');
  },

  toggleBackupConfig() {
    const p = document.getElementById('backup-config-panel');
    if (p) {
      p.style.display = p.style.display === 'none' ? 'block' : 'none';
    }
  },

  async saveBackupConfig() {
    const cfg = {
      endpoint: document.getElementById('s3-endpoint')?.value || '',
      bucket: document.getElementById('s3-bucket')?.value || '',
      region: document.getElementById('s3-region')?.value || 'us-east-1',
      access_key: document.getElementById('s3-access-key')?.value || '',
      secret_key: document.getElementById('s3-secret-key')?.value || '',
    };
    try {
      const res = await fetch('/api/backup/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(cfg)
      });
      if (res.ok) {
        this.data.backupConfig = await res.json();
        this.showToast('Cloud S3 configuration saved successfully');
      } else {
        const err = await res.text();
        this.showToast('Error saving S3 config: ' + err);
      }
    } catch (e) {
      this.showToast('Failed to save S3 config: ' + e.message);
    }
  },

  openCreateBackupModal() {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    if (!modal || !content) return;
    content.innerHTML = `
      <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
        <h3 class="text-lg font-bold text-slate-800">Create Atomic Point-in-Time Snapshot</h3>
        <button class="icon-btn" onclick="app.closeModal()"><i data-lucide="x" class="w-5 h-5"></i></button>
      </div>
      <div style="display:flex; flex-direction:column; gap:14px;">
        <p class="text-xs text-slate-500">
          This operation issues a WAL checkpoint (<code>PRAGMA wal_checkpoint(TRUNCATE)</code>) and creates an atomic snapshot using <code>VACUUM INTO</code> with automatic integrity checking.
        </p>
        <div>
          <label class="form-label">Snapshot Prefix</label>
          <input type="text" id="backup-prefix" class="form-input" value="aosphere_prod">
        </div>
        <div>
          <label class="form-label">Encryption Passphrase (AES-256-GCM) — Leave blank for unencrypted</label>
          <input type="password" id="backup-passphrase" class="form-input" placeholder="Optional encryption passphrase">
        </div>
        <div style="display: flex; align-items: center; gap: 8px;">
          <input type="checkbox" id="backup-upload-cloud" style="width: 16px; height: 16px;">
          <label for="backup-upload-cloud" style="font-size: 13px; font-weight: 500; cursor: pointer;">
            Replicate to configured S3 / Cloudflare R2 immediately
          </label>
        </div>
        <div style="display:flex; justify-content:flex-end; gap:8px; margin-top:8px;">
          <button class="btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn btn-primary" onclick="app.submitCreateBackup()">Take Snapshot</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
    if (window.lucide) lucide.createIcons();
  },

  async submitCreateBackup() {
    const prefix = document.getElementById('backup-prefix')?.value || 'aosphere_backup';
    const passphrase = document.getElementById('backup-passphrase')?.value || '';
    const upload_cloud = document.getElementById('backup-upload-cloud')?.checked || false;

    this.showToast('Initiating WAL checkpoint and atomic snapshot...');
    try {
      const res = await fetch('/api/backup/create', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ prefix, passphrase, upload_cloud })
      });
      if (res.ok) {
        const data = await res.json();
        this.closeModal();
        this.showToast(`Snapshot created: ${data.filename} (${data.size_bytes} bytes)`);
        this.data.backups = null;
        await this.ensureBackupData();
        this.render();
      } else {
        const err = await res.text();
        this.showToast('Backup creation failed: ' + err);
      }
    } catch (e) {
      this.showToast('Network error creating backup: ' + e.message);
    }
  },

  openRestoreBackupModal(filename, isEncrypted) {
    const modal = document.getElementById('action-modal');
    const content = document.getElementById('modal-content');
    if (!modal || !content) return;
    content.innerHTML = `
      <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
        <h3 class="text-lg font-bold text-red-600">Disaster Recovery: Restore Snapshot</h3>
        <button class="icon-btn" onclick="app.closeModal()"><i data-lucide="x" class="w-5 h-5"></i></button>
      </div>
      <div style="display:flex; flex-direction:column; gap:14px;">
        <div style="background: #fef2f2; border: 1px solid #fecaca; padding: 12px; border-radius: 6px; font-size: 13px; color: #991b1b;">
          <strong>Warning:</strong> Restoring this backup will overwrite the current live database file with snapshot <code>${filename}</code>. A checkpoint will be run before restoring.
        </div>
        ${isEncrypted ? `
        <div>
          <label class="form-label">AES-256 Passphrase Required</label>
          <input type="password" id="restore-passphrase" class="form-input" placeholder="Enter encryption passphrase">
        </div>
        ` : ''}
        <div style="display:flex; justify-content:flex-end; gap:8px; margin-top:8px;">
          <button class="btn btn-secondary" onclick="app.closeModal()">Cancel</button>
          <button class="btn btn-danger" onclick="app.submitRestoreBackup('${filename}')">Confirm & Restore</button>
        </div>
      </div>
    `;
    modal.classList.add('active');
    if (window.lucide) lucide.createIcons();
  },

  async submitRestoreBackup(filename) {
    const passphrase = document.getElementById('restore-passphrase')?.value || '';
    this.showToast(`Restoring ${filename}...`);
    try {
      const res = await fetch('/api/backup/restore', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filename, passphrase })
      });
      if (res.ok) {
        this.closeModal();
        this.showToast(`Snapshot ${filename} successfully restored!`);
        setTimeout(() => window.location.reload(), 1500);
      } else {
        const err = await res.text();
        this.showToast('Restore failed: ' + err);
      }
    } catch (e) {
      this.showToast('Restore request failed: ' + e.message);
    }
  },

  async syncBackupCloud(filename) {
    this.showToast(`Uploading ${filename} to Cloud S3 / R2...`);
    try {
      const res = await fetch('/api/backup/cloud-sync', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filename })
      });
      if (res.ok) {
        const data = await res.json();
        this.showToast(`Successfully uploaded to S3: ${data.cloud_key}`);
      } else {
        const err = await res.text();
        this.showToast('Cloud sync error: ' + err);
      }
    } catch (e) {
      this.showToast('Network error syncing to cloud: ' + e.message);
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
