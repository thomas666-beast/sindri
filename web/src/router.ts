import { createRouter, createWebHistory } from 'vue-router';
import { useAuthStore } from './stores/auth';

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { 
      path: '/welcome', 
      name: 'welcome', 
      component: () => import('./pages/WelcomePage.vue'), 
      meta: { public: false } // Requires login
    },

    { path: '/', name: 'home', component: () => import('./pages/HomePage.vue'), meta: { public: true } },
    {
      path: '/login',
      name: 'login',
      component: () => import('./pages/LoginPage.vue'),
      meta: { public: true },
    },
    {
      path: '/wallet',
      name: 'wallet',
      component: () => import('./pages/WalletPage.vue'),
    },
    {
      path: '/products',
      name: 'products',
      component: () => import('./pages/ProductsPage.vue'),
      meta: { public: true },
    },
    {
      path: '/products/new',
      name: 'products-new',
      component: () => import('./pages/ProductNewPage.vue'),
    },
    {
      path: '/products/:id',
      name: 'product-detail',
      component: () => import('./pages/ProductDetailPage.vue'),
      meta: { public: true },
    },
    {
      path: '/orders',
      name: 'orders',
      component: () => import('./pages/OrdersPage.vue'),
    },
    {
      path: '/orders/:id',
      name: 'order-detail',
      component: () => import('./pages/OrderDetailPage.vue'),
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('./pages/RegisterPage.vue'),
      meta: { public: true },
    },
    {
      path: '/invoices',
      name: 'invoices',
      component: () => import('./pages/InvoicesPage.vue'),
    },
    {
      path: '/invoices/:id',
      name: 'invoice-detail',
      component: () => import('./pages/InvoiceDetailPage.vue'),
    },
        {
      path: '/invoices/new',
      name: 'invoices-new',
      component: () => import('./pages/InvoiceNewPage.vue'),
    },
    {
      path: '/admin/withdrawals',
      name: 'admin-withdrawals',
      component: () => import('./pages/AdminWithdrawalsPage.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin',
      name: 'admin',
      component: () => import('./pages/AdminDashboardPage.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/users',
      name: 'admin-users',
      component: () => import('./pages/AdminPage.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/tickets',
      name: 'admin-tickets',
      component: () => import('./pages/AdminTicketsPage.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/users/:id',
      name: 'admin-user',
      component: () => import('./pages/AdminUserDetailPage.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/tickets/:id',
      name: 'admin-ticket',
      component: () => import('./pages/AdminTicketDetailPage.vue'),
      meta: { admin: true },
    },
    {
      path: '/chat',
      name: 'chat',
      component: () => import('./pages/ChatPage.vue'),
    },
    {
      path: '/profile',
      name: 'profile',
      component: () => import('./pages/ProfilePage.vue'),
    },
    {
      path: '/u/:username',
      name: 'public-profile',
      component: () => import('./pages/PublicProfilePage.vue'),
      meta: { public: true },
    },
    {
      path: '/my-products',
      name: 'my-products',
      component: () => import('./pages/MyProductsPage.vue'),
    },
    {
      path: '/products/:id/edit',
      name: 'product-edit',
      component: () => import('./pages/ProductEditPage.vue'),
    },
    {
      path: '/admin/orders',
      name: 'admin-orders',
      component: () => import('./pages/AdminOrdersPage.vue'),
      meta: { admin: true },
    },
    {
      path: '/tickets',
      name: 'tickets',
      component: () => import('./pages/TicketsPage.vue'),
    },
    {
      path: '/tickets/:id',
      name: 'ticket-detail',
      component: () => import('./pages/TicketDetailPage.vue'),
    },
    {
      path: '/geeks',
      name: 'geeks',
      component: () => import('./pages/GeeksPage.vue'),
      meta: { public: true },
    },
    {
      path: '/notifications',
      name: 'notifications',
      component: () => import('./pages/NotificationsPage.vue'),
    },
    {
      path: '/admin/lookup',
      name: 'admin-lookup',
      component: () => import('./pages/AdminOrderLookupPage.vue'),
      meta: { admin: true },
    },
    {
      path: '/dashboard',
      name: 'dashboard',
      component: () => import('./pages/DashboardPage.vue'),
    },
    {
      path: '/admin/transactions',
      name: 'admin-transactions',
      component: () => import('./pages/AdminTransactionsPage.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/settings',
      name: 'admin-settings',
      component: () => import('./pages/AdminSettingsPage.vue'),
      meta: { admin: true },
    }
  ],
});

router.beforeEach(async (to) => {
  const auth = useAuthStore();

  // Wait for the initial /me call to complete before deciding.
  if (!auth.initialized) {
    await auth.refresh();
  }

  if (!auth.user && !to.meta.public) {
    return { name: 'login' };
  }
  if (auth.user && (to.name === 'login' || to.name === 'register')) {
    return { name: 'home' };
  }
});
