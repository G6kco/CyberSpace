-- Rolling back fails while a profile has no programme or graduation year.
ALTER TABLE student_profiles
    MODIFY programme VARCHAR(120) NOT NULL,
    MODIFY graduation_year SMALLINT UNSIGNED NOT NULL;
