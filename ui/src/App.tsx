import { useState } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import './index.css';
import Sidebar from './components/Sidebar';
import Dashboard from './pages/Dashboard';
import Services from './pages/Services';
import RulesPage from './pages/Rules';
import AttackLog from './pages/AttackLog';
import HealthPage from './pages/Health';
import ForensicPage from './pages/Forensic';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { refetchInterval: 5000, staleTime: 3000 },
  },
});

type Page = 'dashboard' | 'services' | 'rules' | 'attacks' | 'health' | 'forensic';

export default function App() {
  const [page, setPage] = useState<Page>('dashboard');

  const renderPage = () => {
    switch (page) {
      case 'dashboard': return <Dashboard />;
      case 'services': return <Services />;
      case 'rules': return <RulesPage />;
      case 'attacks': return <AttackLog />;
      case 'health': return <HealthPage />;
      case 'forensic': return <ForensicPage />;
    }
  };

  return (
    <QueryClientProvider client={queryClient}>
      <div className="flex h-screen overflow-hidden">
        <Sidebar current={page} onNavigate={setPage} />
        <main className="flex-1 overflow-y-auto p-6">
          {renderPage()}
        </main>
      </div>
    </QueryClientProvider>
  );
}
