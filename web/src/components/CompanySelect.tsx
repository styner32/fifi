import { useState, useEffect } from 'react';
import { getFieldValue } from '../types';
import type { Company } from '../types';
import { apiRequest } from '../api';

interface CompanySelectProps {
  onSelect: (company: Company | null) => void;
  selectedCompany: Company | null;
  placeholder?: string;
}

export function CompanySelect({ onSelect, selectedCompany, placeholder = "Search company..." }: CompanySelectProps) {
  const selectedName = selectedCompany
    ? getFieldValue<string>(selectedCompany as unknown as Record<string, unknown>, "CorpName", "corp_name", "name") || ""
    : "";
  const [query, setQuery] = useState(selectedName);
  const [previousSelection, setPreviousSelection] = useState(selectedCompany);
  const [companies, setCompanies] = useState<Company[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const [loading, setLoading] = useState(false);

  // Adjust local input before committing a changed external selection.
  if (previousSelection !== selectedCompany) {
    setPreviousSelection(selectedCompany);
    if (selectedCompany) setQuery(selectedName);
  }

  useEffect(() => {
    const fetchCompanies = async () => {
      setLoading(true);
      try {
        const endpoint = query
          ? `/companies?search=${encodeURIComponent(query)}`
          : "/companies";
        const data = await apiRequest<{ companies: Company[] } | Company[]>(endpoint);
        const list = Array.isArray(data) ? data : data.companies || [];
        setCompanies(list);
      } catch (err) {
        console.error(err);
        setCompanies([]);
      } finally {
        setLoading(false);
      }
    };

    const timer = setTimeout(() => {
        fetchCompanies();
    }, 300);

    return () => clearTimeout(timer);
  }, [query]);

  return (
    <div className="combobox-container">
      <input
        type="text"
        className="combobox-input"
        placeholder={placeholder}
        value={query}
        onChange={(e) => {
            setQuery(e.target.value);
            setIsOpen(true);
            if (!e.target.value) onSelect(null);
        }}
        onFocus={() => setIsOpen(true)}
        // Delay blur to allow click on list item
        onBlur={() => setTimeout(() => setIsOpen(false), 200)}
      />
      {isOpen && (
        <ul className="combobox-list">
          {loading ? (
             <li className="combobox-empty">Loading...</li>
          ) : companies.length === 0 ? (
            <li className="combobox-empty">No companies found</li>
          ) : (
            companies.map((company) => {
               const code = getFieldValue<string>(company as unknown as Record<string, unknown>, "CorpCode", "corp_code", "id") || "";
               const name = getFieldValue<string>(company as unknown as Record<string, unknown>, "CorpName", "corp_name", "name") || "";
               return (
                <li
                  key={code}
                  className={`combobox-item ${selectedCompany && getFieldValue(selectedCompany as unknown as Record<string, unknown>, "CorpCode", "corp_code") === code ? 'selected' : ''}`}
                  onClick={() => {
                    onSelect(company);
                    setQuery(name);
                    setIsOpen(false);
                  }}
                >
                  {name}
                </li>
               );
            })
          )}
        </ul>
      )}
    </div>
  );
}
