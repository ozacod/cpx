import { useState } from 'react';
import { Documentation } from './components/Documentation';
import { ThemeToggle } from './components/ThemeToggle';
import { useTheme } from './contexts/ThemeContext';

type Tab = 'home' | 'docs';

function App() {
  const { theme } = useTheme();
  const [activeTab, setActiveTab] = useState<Tab>('home');
  const [copiedText, setCopiedText] = useState<string | null>(null);

  const copyCommand = (value: string) => {
    navigator.clipboard.writeText(value);
    setCopiedText(value);
    setTimeout(() => setCopiedText(null), 1600);
  };

  const installCommand = 'curl -fsSL https://raw.githubusercontent.com/ozacod/cpx/main/install.sh | sh';

  return (
    <div className={`min-h-screen transition-colors duration-300 ${theme === 'dark' ? 'bg-black' : ''}`} style={theme === 'light' ? { backgroundColor: 'rgb(252, 249, 243)' } : {}}>
      {/* Header */}
      <header className={`border-b backdrop-blur-sm sticky top-0 z-40 ${
        theme === 'dark' 
          ? 'border-white/5 bg-black/20' 
          : 'border-gray-300'
      }`} style={theme === 'light' ? { backgroundColor: 'rgba(252, 249, 243, 0.9)' } : {}}>
        <div className="max-w-screen-xl mx-auto px-6 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-6">
              {/* Logo - clickable to go home */}
              <button 
                onClick={() => setActiveTab('home')}
                className="hover:opacity-80 transition-opacity"
              >
                <img src="/cpx.svg" alt="Cpx" className="w-12 h-12" />
              </button>

              {/* Tabs */}
              <nav className="flex items-center gap-1">
                <button
                  onClick={() => setActiveTab('docs')}
                  className={`px-4 py-2 rounded text-sm font-medium transition-all ${
                    activeTab === 'docs'
                      ? theme === 'dark' 
                        ? 'bg-cyan-500/10 text-cyan-400' 
                        : 'bg-cyan-500/10 text-cyan-600'
                      : theme === 'dark' 
                        ? 'text-gray-400 hover:text-white hover:bg-white/5' 
                        : 'text-gray-600 hover:text-gray-900 hover:bg-gray-100'
                  }`}
                >
                  Documentation
                </button>
              </nav>
            </div>

            {/* Actions */}
            <div className="flex items-center gap-3">
              {/* Theme Toggle */}
              <ThemeToggle />

              {/* GitHub link */}
              <a
                href="https://github.com/ozacod/cpx"
                target="_blank"
                rel="noopener noreferrer"
                className={`px-4 py-2 rounded-lg text-sm font-medium transition-all flex items-center gap-3 ${
                  theme === 'dark'
                    ? 'text-gray-400 hover:text-white hover:bg-white/5'
                    : 'text-gray-600 hover:text-gray-900 hover:bg-gray-100'
                }`}
                title="View on GitHub"
              >
                <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                  <path fillRule="evenodd" clipRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" />
                </svg>
                GitHub
              </a>
            </div>
          </div>
        </div>
      </header>

      {/* Main content */}
      {activeTab === 'docs' ? (
        <Documentation />
      ) : (
        <main className="max-w-screen-xl mx-auto px-6 py-8">
          <div className="flex flex-col items-center justify-center min-h-[70vh] animate-fade-in px-4">
            <div className="max-w-6xl w-full space-y-10">
              <div className="grid gap-10 lg:grid-cols-2 items-center">
                <div className="space-y-10 text-center lg:text-left">
                  <div>
                    <h1 className={`font-display text-6xl md:text-7xl font-bold mb-6 ${
                      theme === 'dark' ? 'text-white' : 'text-gray-900'
                    }`}>
                      The best way to start a C++ project
                    </h1>
                    <p className={`text-lg md:text-xl ${theme === 'dark' ? 'text-gray-300' : 'text-gray-700'}`}>
                      Launch the interactive TUI, answer a few prompts, and get a ready-to-build project with presets, tests, git hooks, and CI scaffolding.
                    </p>
                  </div>

                  <div className="flex gap-4 flex-wrap justify-center lg:justify-start">
                    <button
                      onClick={() => setActiveTab('docs')}
                      className={`px-8 py-3 rounded-lg font-semibold flex items-center gap-2 transition-colors border ${
                        theme === 'dark'
                          ? 'bg-white/10 hover:bg-white/20 text-white border-white/10'
                          : 'bg-gray-100 hover:bg-gray-200 text-gray-900 border-gray-300'
                      }`}
                    >
                      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
                      </svg>
                      View docs
                    </button>
                    <a
                      href="https://github.com/ozacod/cpx"
                      target="_blank"
                      rel="noopener noreferrer"
                      className={`px-8 py-3 rounded-lg font-semibold flex items-center gap-2 transition-colors border ${
                        theme === 'dark'
                          ? 'bg-white/10 hover:bg-white/20 text-white border-white/10'
                          : 'bg-gray-100 hover:bg-gray-200 text-gray-900 border-gray-300'
                      }`}
                    >
                      <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                        <path fillRule="evenodd" clipRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" />
                      </svg>
                      GitHub
                    </a>
                  </div>

                  <div className="space-y-6">
                    <div
                      className={`rounded-2xl border p-5 text-left transition-colors ${
                        theme === 'dark'
                          ? 'bg-white/5 border-white/10 hover:bg-white/10'
                          : 'bg-gray-100 border-gray-300 hover:bg-gray-200'
                      }`}
                    >
                      <div className="flex items-start justify-between gap-4">
                        <code className={`block font-mono text-sm break-all ${
                          theme === 'dark' ? 'text-cyan-400' : 'text-cyan-700'
                        }`}>
                          {installCommand}
                        </code>
                        <button
                          onClick={() => copyCommand(installCommand)}
                          className={`p-2 rounded transition-colors ${
                            theme === 'dark'
                              ? 'bg-white/10 text-white hover:bg-white/20'
                              : 'bg-white text-gray-700 border border-gray-200 hover:bg-gray-50'
                          }`}
                          aria-label="Copy install command"
                          title="Copy install command"
                        >
                          {copiedText === installCommand ? (
                            <svg className="w-4 h-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                            </svg>
                          ) : (
                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                            </svg>
                          )}
                        </button>
                      </div>
                    </div>
                  </div>
                </div>

                <div className={`rounded-2xl border overflow-hidden shadow-sm ${
                  theme === 'dark'
                    ? 'bg-white/5 border-white/10'
                    : 'bg-gray-50 border-gray-200'
                }`}>
                  <img
                    src="/demo.gif"
                    alt="cpx new demo"
                    className="w-full h-full object-cover"
                  />
                </div>
              </div>
            </div>
          </div>
        </main>
      )}
    </div>
  );
}

export default App;
