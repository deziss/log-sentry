import { useState } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider } from './ThemeContext';
import { ToastProvider } from './components/Toast';
import './index.css';
import Sidebar from './components/Sidebar';
import Dashboard from './pages/Dashboard';
import Services from './pages/Services';
import RulesPage from './pages/Rules';
import AttackLog from './pages/AttackLog';
import HealthPage from './pages/Health';
import CrashReports from './pages/CrashReports';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { refetchInterval: 5000, staleTime: 3000 },
  },
});

export type Page = 'dashboard' | 'services' | 'rules' | 'attacks' | 'health' | 'crashes';

export default function App() {
  const [page, setPage] = useState<Page>('dashboard');

  const renderPage = () => {
    switch (page) {
      case 'dashboard': return <Dashboard />;
      case 'services': return <Services />;
      case 'rules': return <RulesPage />;
      case 'attacks': return <AttackLog />;
      case 'health': return <HealthPage />;
      case 'crashes': return <CrashReports />;
    }
  };

  return (
    <ThemeProvider>
      <ToastProvider>
        <QueryClientProvider client={queryClient}>
          <div className="flex h-screen overflow-hidden">
            <Sidebar current={page} onNavigate={setPage} />
            <main className="flex-1 overflow-y-auto p-6 bg-theme-primary">
              {renderPage()}
            </main>
          </div>
        </QueryClientProvider>
      </ToastProvider>
    </ThemeProvider>
  );
}
