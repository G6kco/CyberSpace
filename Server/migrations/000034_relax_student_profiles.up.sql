-- The student list an administrator uploads carries only the register
-- number, name, email and department, so the programme and graduation year
-- become optional.
ALTER TABLE student_profiles
    MODIFY programme VARCHAR(120) NULL,
    MODIFY graduation_year SMALLINT UNSIGNED NULL;
