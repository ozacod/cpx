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
        <div className="max-w-screen-xl mx-auto px-4 md:px-6 py-5">
          <div className="flex items-center justify-between gap-4">
            <div className="flex items-center gap-4 md:gap-8">
              {/* Logo - clickable to go home */}
              <button 
                onClick={() => setActiveTab('home')}
                className="hover:opacity-80 transition-opacity"
              >
                <img src="/cpx.svg" alt="Cpx" className="w-12 h-12 md:w-14 md:h-14" />
              </button>

              {/* Tabs */}
              <nav className="flex items-center gap-1">
                <button
                  onClick={() => setActiveTab('docs')}
                  className={`px-4 md:px-5 py-2 rounded text-sm md:text-base font-medium transition-all ${
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
            <div className="flex items-center gap-3 md:gap-4">
              {/* Theme Toggle */}
              <ThemeToggle />

              {/* GitHub link */}
              <a
                href="https://github.com/ozacod/cpx"
                target="_blank"
                rel="noopener noreferrer"
                className={`px-4 md:px-5 py-2 rounded-lg text-sm md:text-base font-medium transition-all flex items-center gap-2 md:gap-3 ${
                  theme === 'dark'
                    ? 'text-gray-400 hover:text-white hover:bg-white/5'
                    : 'text-gray-600 hover:text-gray-900 hover:bg-gray-100'
                }`}
                title="View on GitHub"
              >
                <svg className="w-5 h-5 md:w-6 md:h-6" fill="currentColor" viewBox="0 0 24 24">
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
        <main className="max-w-6xl mx-auto px-4 md:px-6 pt-16 md:pt-24 pb-12 md:pb-16">
          <div className="flex flex-col items-center justify-center animate-fade-in space-y-12">
            <div className="w-full text-center space-y-6 md:space-y-8">
              {/* Hero heading - T3 style large centered text */}
              <h1 className={`text-4xl sm:text-5xl md:text-6xl font-extrabold tracking-tight ${
                theme === 'dark' ? 'text-white' : 'text-gray-900'
              }`}>
                The best way to start a{' '}
                <span className={theme === 'dark' ? 'text-cyan-400' : 'text-cyan-600'}>
                  C++ project
                </span>
              </h1>

              {/* Buttons row - T3 style */}
              <div className="flex items-center justify-center gap-3 md:gap-4 pt-2 flex-wrap">
                <button
                  onClick={() => setActiveTab('docs')}
                  className={`px-5 md:px-6 py-2.5 md:py-3 rounded-lg text-sm md:text-base font-semibold transition-all ${
                    theme === 'dark'
                      ? 'bg-white text-black hover:bg-gray-200'
                      : 'bg-gray-900 text-white hover:bg-gray-800'
                  }`}
                >
                  Documentation
                </button>
                <a
                  href="https://github.com/ozacod/cpx"
                  target="_blank"
                  rel="noopener noreferrer"
                  className={`px-5 md:px-6 py-2.5 md:py-3 rounded-lg text-sm md:text-base font-semibold transition-all border ${
                    theme === 'dark'
                      ? 'bg-transparent text-white border-white/20 hover:bg-white/10'
                      : 'bg-transparent text-gray-900 border-gray-300 hover:bg-gray-100'
                  }`}
                >
                  GitHub
                </a>
              </div>

              {/* Install command - T3 style code block */}
              <div className="flex justify-center">
                <div
                  onClick={() => copyCommand(installCommand)}
                  className={`group inline-flex items-center gap-3 px-4 md:px-5 py-3 rounded-xl cursor-pointer transition-all ${
                    theme === 'dark'
                      ? 'bg-white/5 hover:bg-white/10 border border-white/10'
                      : 'bg-gray-100 hover:bg-gray-200 border border-gray-200'
                  }`}
                >
                  <code className={`font-mono text-sm ${
                    theme === 'dark' ? 'text-gray-300' : 'text-gray-700'
                  }`}>
                    {installCommand}
                  </code>
                  <span className={`transition-opacity ${
                    copiedText === installCommand ? 'opacity-100' : 'opacity-50 group-hover:opacity-100'
                  }`}>
                    {copiedText === installCommand ? (
                      <svg className="w-4 h-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                      </svg>
                    ) : (
                      <svg className={`w-4 h-4 ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                      </svg>
                    )}
                  </span>
                </div>
              </div>

              {/* Feature 1: TUI Demo */}
              <div className="mt-20 grid md:grid-cols-2 gap-10 md:gap-16 items-center text-left">
                {/* Left side - description */}
                <div className="space-y-4">
                  <h2 className={`text-3xl sm:text-4xl font-bold ${
                    theme === 'dark' ? 'text-white' : 'text-gray-900'
                  }`}>
                    Opinionated From The Start
                  </h2>
                  <p className={`text-lg leading-relaxed ${
                    theme === 'dark' ? 'text-gray-400' : 'text-gray-600'
                  }`}>
                    cpx brings the Go and Rust CLI tool experience to C++. Starting a new C++ project shouldn't mean wrestling with CMake configuration, chasing down dependencies, or debugging build systems. Opinionated templates embrace modern C++ best practices, static compilation is handled via vcpkg package management, and the workflow just works—from day one.
                  </p>
                </div>

                {/* Right side - Demo GIF */}
                <div className={`rounded-2xl border overflow-hidden shadow-lg ${
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

              {/* Feature 2: CMake */}
              <div className="mt-16 grid md:grid-cols-2 gap-10 md:gap-16 items-center text-left">
                {/* Left side - CMake icon */}
                <div className="flex justify-center">
                  <img 
                    src="/cmake.svg" 
                    alt="CMake" 
                    className={`w-32 h-32 ${theme === 'dark' ? 'opacity-80' : 'opacity-90'}`}
                  />
                </div>

                {/* Right side - description */}
                <div className="space-y-4">
                  <h3 className={`text-2xl sm:text-3xl font-bold ${
                    theme === 'dark' ? 'text-white' : 'text-gray-900'
                  }`}>
                    Static Linking with vcpkg
                  </h3>
                  <p className={`text-lg leading-relaxed ${
                    theme === 'dark' ? 'text-gray-400' : 'text-gray-600'
                  }`}>
                    Every project comes pre-configured with CMake, vcpkg integration, unit testing frameworks, code linting, formatting tools, and CI scaffolding for GitHub Actions. Everything is set up to produce statically-linked binaries out of the box, so you can distribute your applications without worrying about shared library dependencies on target systems.
                  </p>
                </div>
              </div>

              {/* Feature 3: Docker Cross-compilation */}
              <div className="mt-16 grid md:grid-cols-2 gap-10 md:gap-16 items-center text-left">
                {/* Left side - description */}
                <div className="space-y-4">
                  <h3 className={`text-2xl sm:text-3xl font-bold ${
                    theme === 'dark' ? 'text-white' : 'text-gray-900'
                  }`}>
                    Cross-Compile Anywhere
                  </h3>
                  <p className={`text-lg leading-relaxed ${
                    theme === 'dark' ? 'text-gray-400' : 'text-gray-600'
                  }`}>
                    Need to cross-compile? Docker images are provided with complete cross-compilation toolchains for Linux (glibc and musl). Binaries can be built for any target platform from any development machine—no need to set up native toolchains or maintain multiple build environments. One command, ship everywhere.
                  </p>
                </div>

                {/* Right side - Docker icon */}
                <div className="flex justify-center">
                  <img 
                    src="/docker.svg" 
                    alt="Docker" 
                    className={`w-48 h-48 ${theme === 'dark' ? 'opacity-80' : 'opacity-90'}`}
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
