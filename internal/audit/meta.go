package audit

// RetentionDays returns configured retention; 0 means keep forever.
func (s *Service) RetentionDays() int {
	if s == nil || s.cfg == nil {
		return 0
	}
	return s.cfg.Audit.RetentionDaysEffective()
}
